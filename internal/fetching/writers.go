package fetching

import (
	"context"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// BuildWriter persists CI builds. Returned int is the row count actually
// written (UpsertMany dedupes by (repo_id, ci_provider, external_id)).
type BuildWriter interface {
	UpsertMany(ctx context.Context, repoID, ciProvider string, builds []build.Build) (int, error)

	// MaxStartedAt returns the largest started_at for the repo and
	// CI provider. Scoping the watermark per provider keeps one
	// provider's progress from advancing another's cursor — a lagging
	// or newly added provider must still walk its own backlog. The
	// `has` flag is false when that provider has no rows yet — that is
	// the cold-start signal the orchestrator branches on. Must stay
	// cheap; called before every sync.
	MaxStartedAt(ctx context.Context, repoID, ciProvider string) (time.Time, bool, error)

	// ListMissingAuthorSHAs returns commit SHAs for the repo whose
	// author_account is still NULL. Used to drive incremental author
	// back-fill so re-runs only fetch logins for rows that still lack one.
	ListMissingAuthorSHAs(ctx context.Context, repoID string) ([]commitsha.SHA, error)

	// UpdateAuthorBySHA updates every build row matching (repo_id, sha)
	// with the resolved login.
	UpdateAuthorBySHA(ctx context.Context, repoID string, sha commitsha.SHA, login string) error
}

// PullRequestWriter persists PRs. UpsertMany writes every column
// including enrichment fields, so a fresh sync of an existing PR
// converges to the upstream-fresh state in a single statement.
type PullRequestWriter interface {
	UpsertMany(ctx context.Context, prs []pullrequest.PullRequest) (int, error)
	FindByNumber(ctx context.Context, repoID string, number int) (pullrequest.PullRequest, error)

	// MaxNumber returns the largest PR number stored for the repo. The
	// `has` flag distinguishes "empty store" from "stored MAX is 0".
	// Orchestrators use it to compute the backfill cursor as
	// max(repo.PRSyncStartNumber, MaxNumber+1).
	MaxNumber(ctx context.Context, repoID string) (n int, has bool, err error)
}

// ReviewWriter persists individual PR review submissions. Upsert is keyed
// on (pull_request_id, reviewer_account, submitted_at).
type ReviewWriter interface {
	Upsert(ctx context.Context, prID string, r pullrequest.Review) error
}
