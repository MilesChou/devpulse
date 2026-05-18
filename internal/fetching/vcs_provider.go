package fetching

import (
	"context"

	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// VCSProvider exposes the subset of GitHub-equivalent APIs the orchestrator
// needs. Implemented by internal/github.
type VCSProvider interface {
	// ListPullRequestsInMonth returns PRs whose PR-created timestamp falls
	// within month. repoID is passed through so the returned PRs can be
	// attached to the right Repo aggregate without an extra lookup.
	ListPullRequestsInMonth(
		ctx context.Context,
		repoID string,
		repoName repo.FullName,
		month MonthRange,
	) ([]pullrequest.PullRequest, error)

	// ListAllPullRequests fetches the full history regardless of month
	// — used for one-off backfills.
	ListAllPullRequests(
		ctx context.Context,
		repoID string,
		repoName repo.FullName,
	) ([]pullrequest.PullRequest, error)

	// GetPullRequest fetches a single PR with detail (additions/deletions).
	GetPullRequest(
		ctx context.Context,
		repoID string,
		repoName repo.FullName,
		number int,
	) (pullrequest.PullRequest, error)

	// ListReviews returns every review on the PR. Filtering against
	// ready_at is the orchestrator's job, not the provider's.
	ListReviews(
		ctx context.Context,
		repoName repo.FullName,
		number int,
	) ([]pullrequest.Review, error)

	// GetCommitAuthorAccountsBulk resolves the GitHub login for each SHA
	// in a single batched call. Map value is *string so a known-unknown
	// SHA (resolved but no author) is distinguishable from a missing key.
	GetCommitAuthorAccountsBulk(
		ctx context.Context,
		repoName repo.FullName,
		shas []commitsha.SHA,
	) (map[commitsha.SHA]*string, error)
}
