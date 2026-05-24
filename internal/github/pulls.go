package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/timex"
)

// rawPull is the slim subset of GitHub's REST PR JSON we read. Extend as
// new fields become relevant; unknown JSON keys are tolerated.
type rawPull struct {
	Number    int        `json:"number"`
	State     string     `json:"state"` // open / closed
	Draft     bool       `json:"draft"`
	Title     string     `json:"title"`
	User      *rawUser   `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
	ClosedAt  *time.Time `json:"closed_at"`

	// PR detail-only fields. Absent from the /pulls list endpoint.
	Additions    int `json:"additions,omitempty"`
	Deletions    int `json:"deletions,omitempty"`
	ChangedLines int `json:"changed_files,omitempty"`
}

type rawUser struct {
	Login string `json:"login"`
}

// toDomain converts a rawPull to the domain model. repoID flows in so
// the caller can attach the PR to the right repo aggregate.
func (r rawPull) toDomain(repoID string) pullrequest.PullRequest {
	author := ""
	if r.User != nil {
		author = r.User.Login
	}

	status := pullrequest.StatusOpen
	switch {
	case r.MergedAt != nil:
		status = pullrequest.StatusMerged
	case strings.EqualFold(r.State, "closed"):
		status = pullrequest.StatusClosed
	}

	pr := pullrequest.PullRequest{
		RepoID:            repoID,
		Number:            r.Number,
		Author:            author,
		Status:            status,
		Additions:         r.Additions,
		Deletions:         r.Deletions,
		TotalChangedLines: r.Additions + r.Deletions,
		IsDraft:           r.Draft,
		CreatedAt:         r.CreatedAt.UTC(),
		MergedAt:          timex.PtrUTC(r.MergedAt),
		ClosedAt:          timex.PtrUTC(r.ClosedAt),
	}

	// GitHub's REST does not surface ready_at directly. For non-draft PRs,
	// approximate it with CreatedAt — accurate for the common case. A
	// dedicated enrichment pass (via GraphQL timelineItems) is the correct
	// fix but stays out of v1 to limit scope.
	if !r.Draft {
		t := r.CreatedAt.UTC()
		pr.ReadyAt = &t
	}
	return pr
}

// listPullsPage returns one page worth of PRs. The new by-number sync
// flow only ever needs the first page to read the latest PR number,
// so pagination state (Link header) is intentionally not tracked here.
// Sort=created, direction=desc, state=all.
func (c *Client) listPullsPage(
	ctx context.Context,
	repoName repo.FullName,
	page, perPage int,
) ([]rawPull, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", repoName.Owner, repoName.Name)
	q := url.Values{}
	q.Set("state", "all")
	q.Set("sort", "created")
	q.Set("direction", "desc")
	q.Set("per_page", strconv.Itoa(perPage))
	q.Set("page", strconv.Itoa(page))

	var batch []rawPull
	if _, err := c.rest(ctx, "GET", path, q, &batch); err != nil {
		return nil, err
	}
	return batch, nil
}

// GetPullRequest fetches a single PR with detail (additions/deletions).
// Returns an error wrapping ErrNotFound when the upstream responds 404
// (the number belongs to an issue, was deleted, or never existed).
func (c *Client) GetPullRequest(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
	number int,
) (pullrequest.PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", repoName.Owner, repoName.Name, number)

	var raw rawPull
	if _, err := c.rest(ctx, "GET", path, nil, &raw); err != nil {
		return pullrequest.PullRequest{}, err
	}
	return raw.toDomain(repoID), nil
}

// GetLatestPRNumber returns the highest PR number currently in the repo.
// It pulls the first page of the sort=created direction=desc list and
// reads the first entry's number — one REST call regardless of repo
// size.
//
// Assumption: GitHub assigns PR numbers monotonically at creation time,
// so "newest by created_at" coincides with "highest number". This is
// not formally documented but holds in practice; if upstream ever
// changes, the by-number backfill upper bound would skip the tail of
// any window where a new PR was opened between this call and the loop
// end — that PR would still be picked up on the next sync via the
// db_max+1 cursor.
//
// Returns 0 with no error when the repo has zero PRs.
func (c *Client) GetLatestPRNumber(
	ctx context.Context,
	repoName repo.FullName,
) (int, error) {
	batch, err := c.listPullsPage(ctx, repoName, 1, 1)
	if err != nil {
		return 0, fmt.Errorf("get latest pr number: %w", err)
	}
	if len(batch) == 0 {
		return 0, nil
	}
	return batch[0].Number, nil
}
