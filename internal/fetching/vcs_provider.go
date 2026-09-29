package fetching

import (
	"context"
	"time"

	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// VCSProvider exposes the subset of GitHub-equivalent APIs the orchestrator
// needs. Implemented by internal/github.
type VCSProvider interface {
	// GetPullRequest fetches a single PR with detail (additions/deletions).
	// Implementations must wrap upstream 404 in a way that callers can
	// detect via errors.Is (e.g. github.ErrNotFound) so the backfill loop
	// can distinguish "this number is an issue" from real failures.
	GetPullRequest(
		ctx context.Context,
		repoID string,
		repoName repo.FullName,
		number int,
	) (pullrequest.PullRequest, error)

	// GetLatestPRNumber returns the largest PR number currently visible
	// upstream. Used as the upper bound of the by-number backfill loop.
	// Returns 0 when the repo has no PRs.
	GetLatestPRNumber(
		ctx context.Context,
		repoName repo.FullName,
	) (int, error)

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

	// GetRepo fetches repo metadata (description, default branch,
	// disabled). The returned Repo has its ID unset — the caller overlays
	// it on the persisted aggregate.
	GetRepo(
		ctx context.Context,
		repoName repo.FullName,
	) (repo.Repo, error)

	// ListPullRequestsUpdatedSince returns the number and upstream
	// updated_at of every PR updated at or after since, and newest: the
	// updated_at of the most recently updated PR in the repo (zero when
	// it has none). A zero since returns only newest, without walking
	// the history. newest comes from the same listing, so a stale
	// (cached) listing can never move the caller's watermark past
	// updates it did not return.
	ListPullRequestsUpdatedSince(
		ctx context.Context,
		repoName repo.FullName,
		since time.Time,
	) (updated []pullrequest.Stamp, newest time.Time, err error)

	// GetFirstCommitAt returns the earliest commit author time among the
	// PR's commits (DORA lead-time start), or nil when none is known.
	GetFirstCommitAt(
		ctx context.Context,
		repoName repo.FullName,
		number int,
	) (*time.Time, error)

	// ListIncidentIssues returns every issue — open and closed, pull
	// requests excluded — carrying label. The result is complete or an
	// error: callers replace the stored set with it.
	ListIncidentIssues(
		ctx context.Context,
		repoName repo.FullName,
		label string,
	) ([]incident.Incident, error)
}
