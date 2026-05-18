package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/pullrequest"
)

// PullRequestPersister implements fetching.PullRequestWriter.
type PullRequestPersister struct{ *Persister }

func NewPullRequestPersister(p *Persister) *PullRequestPersister {
	return &PullRequestPersister{Persister: p}
}

var ErrPullRequestNotFound = errors.New("persistence: pull request not found")

// UpsertMany inserts each PR, updating mutable lifecycle fields on conflict
// of (repo_id, number). Enrichment fields are not touched here — those are
// owned by UpdateEnrichment so a re-import never blows away derived data.
func (r *PullRequestPersister) UpsertMany(ctx context.Context, prs []pullrequest.PullRequest) (int, error) {
	if len(prs) == 0 {
		return 0, nil
	}

	insert := r.Rebind(r.upsertSQL())

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("pr upsert begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var written int
	for i := range prs {
		p := prs[i]
		id := p.ID
		if id == "" {
			id = r.NewID()
		}
		now := r.Now()

		res, err := tx.ExecContext(ctx, insert,
			id, "github", p.RepoID, p.Number, p.Author, p.Status.String(),
			p.Additions, p.Deletions, p.TotalChangedLines,
			p.IsDraft, p.CreatedAt, p.ReadyAt, p.MergedAt, p.ClosedAt,
			now, now,
		)
		if err != nil {
			return written, fmt.Errorf("pr upsert row: %w", err)
		}
		n, _ := res.RowsAffected()
		written += int(n)
	}

	if err := tx.Commit(); err != nil {
		return written, fmt.Errorf("pr upsert commit: %w", err)
	}
	return written, nil
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

// ListInMonth returns PRs whose pr_created_at lies inside [month.Start, month.End).
func (r *PullRequestPersister) ListInMonth(ctx context.Context, repoID string, month fetching.MonthRange) ([]pullrequest.PullRequest, error) {
	const q = `SELECT id, repo_id, number, author_account, status,
	                  additions, deletions, total_changed_lines, is_draft,
	                  pr_created_at, ready_at, first_review_at, first_approved_at,
	                  time_to_approval, time_to_merge, merged_at, closed_at
	             FROM pull_requests
	            WHERE repo_id = ?
	              AND pr_created_at >= ?
	              AND pr_created_at <  ?
	            ORDER BY pr_created_at`

	rows, err := r.QueryCtx(ctx, q, repoID, month.Start, month.End)
	if err != nil {
		return nil, fmt.Errorf("pr list in month: %w", err)
	}
	defer rows.Close()

	out := make([]pullrequest.PullRequest, 0, 16)
	for rows.Next() {
		got, err := scanPullRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, got)
	}
	return out, rows.Err()
}

// UpdateEnrichment writes every enrichment-derived field. No fillable
// gymnastics: every column in the patch is listed in SET, period.
func (r *PullRequestPersister) UpdateEnrichment(ctx context.Context, prID string, patch pullrequest.EnrichmentPatch) error {
	const q = `UPDATE pull_requests
	             SET additions           = ?,
	                 deletions           = ?,
	                 total_changed_lines = ?,
	                 first_review_at     = ?,
	                 first_approved_at   = ?,
	                 time_to_approval    = ?,
	                 time_to_merge       = ?,
	                 updated_at          = ?
	           WHERE id = ?`

	_, err := r.ExecCtx(ctx, q,
		patch.Additions, patch.Deletions, patch.TotalChangedLines,
		patch.FirstReviewAt, patch.FirstApprovedAt,
		patch.TimeToApproval, patch.TimeToMerge,
		r.Now(), prID,
	)
	if err != nil {
		return fmt.Errorf("pr update enrichment: %w", err)
	}
	return nil
}

// upsertSQL returns the dialect-appropriate UPSERT. Enrichment columns
// are deliberately untouched in DO UPDATE.
func (r *PullRequestPersister) upsertSQL() string {
	cols := `id, platform, repo_id, number, author_account, status,
	         additions, deletions, total_changed_lines,
	         is_draft, pr_created_at, ready_at, merged_at, closed_at,
	         created_at, updated_at`

	values := `?, ?, ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?, ?, ?, ?,
	           ?, ?`

	switch r.Dialect.String() {
	case "mysql":
		return `INSERT INTO pull_requests (` + cols + `) VALUES (` + values + `)
		        ON DUPLICATE KEY UPDATE
		            status        = VALUES(status),
		            is_draft      = VALUES(is_draft),
		            ready_at      = VALUES(ready_at),
		            merged_at     = VALUES(merged_at),
		            closed_at     = VALUES(closed_at),
		            updated_at    = VALUES(updated_at)`
	default:
		return `INSERT INTO pull_requests (` + cols + `) VALUES (` + values + `)
		        ON CONFLICT (repo_id, number) DO UPDATE SET
		            status        = EXCLUDED.status,
		            is_draft      = EXCLUDED.is_draft,
		            ready_at      = EXCLUDED.ready_at,
		            merged_at     = EXCLUDED.merged_at,
		            closed_at     = EXCLUDED.closed_at,
		            updated_at    = EXCLUDED.updated_at`
	}
}

// scanPullRequest decodes one row from the standard SELECT column list.
type rowScanner interface {
	Scan(dest ...any) error
}

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

	switch statusStr {
	case "open":
		p.Status = pullrequest.StatusOpen
	case "merged":
		p.Status = pullrequest.StatusMerged
	case "closed":
		p.Status = pullrequest.StatusClosed
	default:
		p.Status = pullrequest.StatusUnknown
	}
	return p, nil
}

