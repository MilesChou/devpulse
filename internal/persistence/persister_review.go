package persistence

import (
	"context"
	"fmt"

	"github.com/mileschou/devpulse/internal/pullrequest"
)

// ReviewPersister implements fetching.ReviewWriter.
type ReviewPersister struct{ *Persister }

func NewReviewPersister(p *Persister) *ReviewPersister { return &ReviewPersister{Persister: p} }

// Upsert inserts the review row; on conflict of
// (pull_request_id, reviewer_account, submitted_at) it refreshes only the
// `state` column (a re-fetch can promote commented -> approved, etc.).
func (r *ReviewPersister) Upsert(ctx context.Context, prID string, rev pullrequest.Review) error {
	id := r.NewID()
	now := r.Now()

	q := r.Rebind(r.upsertSQL())

	_, err := r.DB.ExecContext(ctx, q,
		id, prID, rev.ReviewerAccount, rev.State.String(), rev.SubmittedAt,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("review upsert: %w", err)
	}
	return nil
}

func (r *ReviewPersister) upsertSQL() string {
	cols := `id, pull_request_id, reviewer_account, state, submitted_at, created_at, updated_at`
	values := `?, ?, ?, ?, ?, ?, ?`

	switch r.Dialect.String() {
	case "mysql":
		return `INSERT INTO pull_request_reviews (` + cols + `) VALUES (` + values + `)
		        ON DUPLICATE KEY UPDATE state = VALUES(state), updated_at = VALUES(updated_at)`
	default:
		return `INSERT INTO pull_request_reviews (` + cols + `) VALUES (` + values + `)
		        ON CONFLICT (pull_request_id, reviewer_account, submitted_at) DO UPDATE SET
		            state      = EXCLUDED.state,
		            updated_at = EXCLUDED.updated_at`
	}
}
