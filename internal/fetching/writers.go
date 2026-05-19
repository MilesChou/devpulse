package fetching

import (
	"context"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// BuildWriter persists CI builds. Returned int is the row count actually
// written (UpsertMany dedupes by (repo_id, commit_sha, number)).
type BuildWriter interface {
	UpsertMany(ctx context.Context, repoID string, builds []build.Build) (int, error)

	// ListMissingAuthorSHAs returns SHAs in `month` whose author_account
	// is still NULL — these are what GetCommitAuthorAccountsBulk needs.
	ListMissingAuthorSHAs(ctx context.Context, repoID string, month MonthRange) ([]commitsha.SHA, error)

	// UpdateAuthorBySHA updates every build row matching (repo_id, sha)
	// with the resolved login.
	UpdateAuthorBySHA(ctx context.Context, repoID string, sha commitsha.SHA, login string) error
}

// PullRequestWriter persists PRs and their enrichment patches.
type PullRequestWriter interface {
	UpsertMany(ctx context.Context, prs []pullrequest.PullRequest) (int, error)
	FindByNumber(ctx context.Context, repoID string, number int) (pullrequest.PullRequest, error)
	ListInMonth(ctx context.Context, repoID string, month MonthRange) ([]pullrequest.PullRequest, error)
	UpdateEnrichment(ctx context.Context, prID string, patch pullrequest.EnrichmentPatch) error
}

// ReviewWriter persists individual PR review submissions. Upsert is keyed
// on (pull_request_id, reviewer_account, submitted_at).
type ReviewWriter interface {
	Upsert(ctx context.Context, prID string, r pullrequest.Review) error
}
