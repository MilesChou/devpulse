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

// ListAllPullRequests pages through every PR with no time cutoff.
func (c *Client) ListAllPullRequests(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
) ([]pullrequest.PullRequest, error) {
	var out []pullrequest.PullRequest
	err := c.ListAllPullRequestsPageFunc(ctx, repoID, repoName, func(page []pullrequest.PullRequest) error {
		out = append(out, page...)
		return nil
	})
	return out, err
}

// ListAllPullRequestsPageFunc pages through every PR, calling fn per page.
func (c *Client) ListAllPullRequestsPageFunc(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
	fn func(page []pullrequest.PullRequest) error,
) error {
	page := 1
	for {
		batch, more, err := c.listPullsPage(ctx, repoName, page, defaultPerPage)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}

		prs := make([]pullrequest.PullRequest, 0, len(batch))
		for _, raw := range batch {
			prs = append(prs, raw.toDomain(repoID))
		}
		if err := fn(prs); err != nil {
			return err
		}

		if !more {
			return nil
		}
		page++
	}
}

// listPullsPage returns one page worth of PRs plus a "more pages" flag
// derived from the Link header. Sort=created, direction=desc, state=all.
func (c *Client) listPullsPage(
	ctx context.Context,
	repoName repo.FullName,
	page, perPage int,
) ([]rawPull, bool, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", repoName.Owner, repoName.Name)
	q := url.Values{}
	q.Set("state", "all")
	q.Set("sort", "created")
	q.Set("direction", "desc")
	q.Set("per_page", strconv.Itoa(perPage))
	q.Set("page", strconv.Itoa(page))

	var batch []rawPull
	hdr, err := c.rest(ctx, "GET", path, q, &batch)
	if err != nil {
		return nil, false, err
	}
	return batch, hasNextPage(hdr.Get("Link")), nil
}

// hasNextPage parses GitHub's Link header for a rel="next" segment.
func hasNextPage(link string) bool {
	return strings.Contains(link, `rel="next"`)
}

// GetPullRequest fetches a single PR with detail (additions/deletions).
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
