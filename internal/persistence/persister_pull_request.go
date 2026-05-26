package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

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
			p.Additions, p.Deletions, p.TotalChangedLines,
			p.IsDraft, p.CreatedAt, p.ReadyAt,
			p.FirstReviewAt, p.FirstApprovedAt,
			p.TimeToApproval, p.TimeToMerge,
			p.MergedAt, p.ClosedAt,
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

// FindByNumber returns the PR by (repo_id, number).
func (r *PullRequestPersister) FindByNumber(ctx context.Context, repoID string, number int) (pullrequest.PullRequest, error) {
	const q = `SELECT id, repo_id, number, author_account, status,
	                  additions, deletions, total_changed_lines, is_draft,
	                  pr_created_at, ready_at, first_review_at, first_approved_at,
	                  time_to_approval, time_to_merge, merged_at, closed_at
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
	         additions, deletions, total_changed_lines,
	         is_draft, pr_created_at, ready_at,
	         first_review_at, first_approved_at,
	         time_to_approval, time_to_merge,
	         merged_at, closed_at,
	         created_at, updated_at`

	values := `?, ?, ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?, ?,
	           ?, ?,
	           ?, ?,
	           ?, ?,
	           ?, ?`

	if r.Dialect.IsMySQL() {
		return `INSERT INTO pull_requests (` + cols + `) VALUES (` + values + `)
		        ON DUPLICATE KEY UPDATE
		            status              = VALUES(status),
		            additions           = VALUES(additions),
		            deletions           = VALUES(deletions),
		            total_changed_lines = VALUES(total_changed_lines),
		            is_draft            = VALUES(is_draft),
		            ready_at            = VALUES(ready_at),
		            first_review_at     = VALUES(first_review_at),
		            first_approved_at   = VALUES(first_approved_at),
		            time_to_approval    = VALUES(time_to_approval),
		            time_to_merge       = VALUES(time_to_merge),
		            merged_at           = VALUES(merged_at),
		            closed_at           = VALUES(closed_at),
		            updated_at          = VALUES(updated_at)`
	}
	return `INSERT INTO pull_requests (` + cols + `) VALUES (` + values + `)
	        ON CONFLICT (repo_id, number) DO UPDATE SET
	            status              = EXCLUDED.status,
	            additions           = EXCLUDED.additions,
	            deletions           = EXCLUDED.deletions,
	            total_changed_lines = EXCLUDED.total_changed_lines,
	            is_draft            = EXCLUDED.is_draft,
	            ready_at            = EXCLUDED.ready_at,
	            first_review_at     = EXCLUDED.first_review_at,
	            first_approved_at   = EXCLUDED.first_approved_at,
	            time_to_approval    = EXCLUDED.time_to_approval,
	            time_to_merge       = EXCLUDED.time_to_merge,
	            merged_at           = EXCLUDED.merged_at,
	            closed_at           = EXCLUDED.closed_at,
	            updated_at          = EXCLUDED.updated_at`
}

// scanPullRequest decodes one row from the standard SELECT column list.
func scanPullRequest(s rowScanner) (pullrequest.PullRequest, error) {
	var p pullrequest.PullRequest
	var statusStr string

	err := s.Scan(
		&p.ID, &p.RepoID, &p.Number, &p.Author, &statusStr,
		&p.Additions, &p.Deletions, &p.TotalChangedLines, &p.IsDraft,
		&p.CreatedAt, &p.ReadyAt, &p.FirstReviewAt, &p.FirstApprovedAt,
		&p.TimeToApproval, &p.TimeToMerge, &p.MergedAt, &p.ClosedAt,
	)
	if err != nil {
		return p, err
	}

	p.Status = pullrequest.ParseStatus(statusStr)
	return p, nil
}
