package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mileschou/devpulse/internal/pullrequest"
)

// PullRequestPersister implements fetching.PullRequestWriter.
type PullRequestPersister struct{ *Persister }

func NewPullRequestPersister(p *Persister) *PullRequestPersister {
	return &PullRequestPersister{Persister: p}
}

var ErrPullRequestNotFound = errors.New("persistence: pull request not found")

// UpsertMany inserts each PR with every field including enrichment,
// updating every mutable column on conflict of (repo_id, number). The
// caller is responsible for populating the row in full (basic fields +
// additions/deletions + first_review_at / first_approved_at /
// time_to_approval / time_to_merge) before calling — see the
// orchestrator's syncOnePullRequestByNumber for the canonical assembly.
//
// Conflict-path behavior: every column except the immutable identifiers
// (id, platform, repo_id, number, pr_created_at, author_account) is
// overwritten with the incoming value. The "re-import won't blow away
// enrichment" guard from earlier versions is gone because the new flow
// guarantees the incoming row already carries fresh enrichment — see
// the doc on syncOnePullRequestByNumber for why that's safe.
//
// For new rows the generated id is written back into prs[i].ID so
// callers can drive follow-up writes (reviews) without re-querying. For
// pre-existing rows the persisted id is looked up post-commit so the
// caller always observes the canonical DB id.
func (r *PullRequestPersister) UpsertMany(ctx context.Context, prs []pullrequest.PullRequest) (int, error) {
	if len(prs) == 0 {
		return 0, nil
	}

	insert := r.Rebind(r.upsertSQL())
	lookup := r.Rebind(`SELECT id FROM pull_requests WHERE repo_id = ? AND number = ?`)

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("pr upsert begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var written int
	for i := range prs {
		p := &prs[i]
		if p.ID == "" {
			p.ID = r.NewID()
		}
		now := r.Now()

		args := []any{
			p.ID, "github", p.RepoID, p.Number, p.Author, p.Status.String(),
			p.Additions, p.Deletions, p.TotalChangedLines, p.SizeBucket,
			p.IsDraft, p.CreatedAt, p.ReadyAt,
			p.FirstReviewAt, p.FirstApprovedAt,
			p.TimeToApproval, p.TimeToMerge,
			p.MergedAt, p.ClosedAt,
			nullableText(p.Title), encodeLabels(p.Labels),
			nullableText(p.BaseRef), nullableText(p.HeadRef),
			nullableText(p.MergeCommitSHA), p.FirstCommitAt, p.RevertsNumber,
			now, now,
		}
		r.Logger.Debug("sql.exec", slog.String("query", r.upsertSQL()), slog.Any("args", args))
		res, err := tx.ExecContext(ctx, insert, args...)
		if err != nil {
			return written, fmt.Errorf("pr upsert row: %w", err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			written += int(n)
			continue
		}

		// Conflict path: the row already existed. Our locally-generated id
		// was ignored; load the persisted one so the caller can drive
		// follow-up writes against the canonical row.
		var existingID string
		r.Logger.Debug("sql.query_row", slog.String("query", `SELECT id FROM pull_requests WHERE repo_id = ? AND number = ?`), slog.Any("args", []any{p.RepoID, p.Number}))
		if err := tx.QueryRowContext(ctx, lookup, p.RepoID, p.Number).Scan(&existingID); err != nil {
			return written, fmt.Errorf("pr upsert lookup id: %w", err)
		}
		p.ID = existingID
	}

	if err := tx.Commit(); err != nil {
		return written, fmt.Errorf("pr upsert commit: %w", err)
	}
	return written, nil
}

// MaxNumber returns the largest PR number persisted for the repo, along
// with a `has` flag distinguishing "empty store" from "stored MAX is 0".
// It is the sync orchestrator's derived cursor: the next backfill round
// resumes at max(repo.PRSyncStartNumber, MaxNumber+1), so the call
// MUST stay cheap and side-effect-free.
//
// Returns (0, false, nil) when the repo has no PRs yet.
func (r *PullRequestPersister) MaxNumber(ctx context.Context, repoID string) (int, bool, error) {
	const q = `SELECT MAX(number) FROM pull_requests WHERE repo_id = ?`

	var n sql.NullInt64
	if err := r.QueryRowCtx(ctx, q, repoID).Scan(&n); err != nil {
		return 0, false, fmt.Errorf("pr max number: %w", err)
	}
	if !n.Valid {
		return 0, false, nil
	}
	return int(n.Int64), true, nil
}

// ListOpenNumbers returns, ascending, the numbers of every PR stored as
// open for the repo. The orchestrator refreshes these on every sync, so
// a stale (cached) open copy heals once its cache entry expires.
func (r *PullRequestPersister) ListOpenNumbers(ctx context.Context, repoID string) ([]int, error) {
	const q = `SELECT number FROM pull_requests
	           WHERE repo_id = ? AND status = 'open'
	           ORDER BY number`

	out, err := r.queryNumbers(ctx, q, repoID)
	if err != nil {
		return nil, fmt.Errorf("pr list open: %w", err)
	}
	return out, nil
}

// ListIncompleteNumbers returns, ascending, the numbers of every PR
// stored without its DORA facts: no base_ref (every current sync stores
// one, so the row predates the DORA columns), or merged with no
// first_commit_at. The orchestrator re-syncs them until they are
// complete.
func (r *PullRequestPersister) ListIncompleteNumbers(ctx context.Context, repoID string) ([]int, error) {
	const q = `SELECT number FROM pull_requests
	           WHERE repo_id = ?
	             AND (base_ref IS NULL
	                  OR (status = 'merged' AND first_commit_at IS NULL))
	           ORDER BY number`

	out, err := r.queryNumbers(ctx, q, repoID)
	if err != nil {
		return nil, fmt.Errorf("pr list incomplete: %w", err)
	}
	return out, nil
}

func (r *PullRequestPersister) queryNumbers(ctx context.Context, q string, args ...any) ([]int, error) {
	rows, err := r.QueryCtx(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// FindByNumber returns the PR by (repo_id, number).
func (r *PullRequestPersister) FindByNumber(ctx context.Context, repoID string, number int) (pullrequest.PullRequest, error) {
	const q = `SELECT id, repo_id, number, author_account, status,
	                  additions, deletions, total_changed_lines, size_bucket, is_draft,
	                  pr_created_at, ready_at, first_review_at, first_approved_at,
	                  time_to_approval, time_to_merge, merged_at, closed_at,
	                  title, labels, base_ref, head_ref, merge_commit_sha,
	                  first_commit_at, reverts_number
	             FROM pull_requests WHERE repo_id = ? AND number = ?`

	row := r.QueryRowCtx(ctx, q, repoID, number)
	got, err := scanPullRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return pullrequest.PullRequest{}, ErrPullRequestNotFound
	}
	return got, err
}

// upsertSQL returns the dialect-appropriate UPSERT. Every mutable
// column is updated on conflict — including additions/deletions and
// enrichment timestamps — so a re-sync of the same PR number always
// converges to the upstream-fresh state.
//
// Columns intentionally NOT updated on conflict:
//   - id, platform, repo_id, number — identifiers
//   - author_account, pr_created_at — immutable historical facts
//   - created_at — row insertion time, distinct from updated_at
func (r *PullRequestPersister) upsertSQL() string {
	cols := `id, platform, repo_id, number, author_account, status,
	         additions, deletions, total_changed_lines, size_bucket,
	         is_draft, pr_created_at, ready_at,
	         first_review_at, first_approved_at,
	         time_to_approval, time_to_merge,
	         merged_at, closed_at,
	         title, labels, base_ref, head_ref,
	         merge_commit_sha, first_commit_at, reverts_number,
	         created_at, updated_at`

	values := `?, ?, ?, ?, ?, ?,
	           ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?,
	           ?, ?,
	           ?, ?,
	           ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?`

	if r.Dialect.IsMySQL() {
		return `INSERT INTO pull_requests (` + cols + `) VALUES (` + values + `)
		        ON DUPLICATE KEY UPDATE
		            status              = VALUES(status),
		            additions           = VALUES(additions),
		            deletions           = VALUES(deletions),
		            total_changed_lines = VALUES(total_changed_lines),
		            size_bucket         = VALUES(size_bucket),
		            is_draft            = VALUES(is_draft),
		            ready_at            = VALUES(ready_at),
		            first_review_at     = VALUES(first_review_at),
		            first_approved_at   = VALUES(first_approved_at),
		            time_to_approval    = VALUES(time_to_approval),
		            time_to_merge       = VALUES(time_to_merge),
		            merged_at           = VALUES(merged_at),
		            closed_at           = VALUES(closed_at),
		            title               = VALUES(title),
		            labels              = VALUES(labels),
		            base_ref            = VALUES(base_ref),
		            head_ref            = VALUES(head_ref),
		            merge_commit_sha    = VALUES(merge_commit_sha),
		            first_commit_at     = VALUES(first_commit_at),
		            reverts_number      = VALUES(reverts_number),
		            updated_at          = VALUES(updated_at)`
	}
	return `INSERT INTO pull_requests (` + cols + `) VALUES (` + values + `)
	        ON CONFLICT (repo_id, number) DO UPDATE SET
	            status              = EXCLUDED.status,
	            additions           = EXCLUDED.additions,
	            deletions           = EXCLUDED.deletions,
	            total_changed_lines = EXCLUDED.total_changed_lines,
	            size_bucket         = EXCLUDED.size_bucket,
	            is_draft            = EXCLUDED.is_draft,
	            ready_at            = EXCLUDED.ready_at,
	            first_review_at     = EXCLUDED.first_review_at,
	            first_approved_at   = EXCLUDED.first_approved_at,
	            time_to_approval    = EXCLUDED.time_to_approval,
	            time_to_merge       = EXCLUDED.time_to_merge,
	            merged_at           = EXCLUDED.merged_at,
	            closed_at           = EXCLUDED.closed_at,
	            title               = EXCLUDED.title,
	            labels              = EXCLUDED.labels,
	            base_ref            = EXCLUDED.base_ref,
	            head_ref            = EXCLUDED.head_ref,
	            merge_commit_sha    = EXCLUDED.merge_commit_sha,
	            first_commit_at     = EXCLUDED.first_commit_at,
	            reverts_number      = EXCLUDED.reverts_number,
	            updated_at          = EXCLUDED.updated_at`
}

// scanPullRequest decodes one row from the standard SELECT column list.
func scanPullRequest(s rowScanner) (pullrequest.PullRequest, error) {
	var p pullrequest.PullRequest
	var statusStr string
	var title, labels, baseRef, headRef, mergeSHA sql.NullString

	err := s.Scan(
		&p.ID, &p.RepoID, &p.Number, &p.Author, &statusStr,
		&p.Additions, &p.Deletions, &p.TotalChangedLines, &p.SizeBucket, &p.IsDraft,
		&p.CreatedAt, &p.ReadyAt, &p.FirstReviewAt, &p.FirstApprovedAt,
		&p.TimeToApproval, &p.TimeToMerge, &p.MergedAt, &p.ClosedAt,
		&title, &labels, &baseRef, &headRef, &mergeSHA,
		&p.FirstCommitAt, &p.RevertsNumber,
	)
	if err != nil {
		return p, err
	}

	p.Status = pullrequest.ParseStatus(statusStr)
	p.Title = title.String
	p.Labels = decodeLabels(labels.String)
	p.BaseRef = baseRef.String
	p.HeadRef = headRef.String
	p.MergeCommitSHA = mergeSHA.String
	return p, nil
}

// labelSeparator joins labels into the single pull_requests.labels
// column. GitHub label names cannot contain a newline, so the encoding
// is unambiguous without escaping.
const labelSeparator = "\n"

// encodeLabels stores an empty label set as NULL.
func encodeLabels(labels []string) any {
	if len(labels) == 0 {
		return nil
	}
	return strings.Join(labels, labelSeparator)
}

func decodeLabels(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, labelSeparator)
}

// nullableText stores an empty string as NULL, so rows synced before a
// column existed and rows with no value read back the same way.
func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}
