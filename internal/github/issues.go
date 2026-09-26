package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/timex"
)

const issuesPerPage = 100

// rawIssue is the slim subset of GitHub's REST issue JSON we read. The
// issues endpoint also returns pull requests; those carry a non-nil
// pull_request key.
type rawIssue struct {
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	CreatedAt   time.Time  `json:"created_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	PullRequest *struct{}  `json:"pull_request"`
}

// ListIncidentIssues returns every issue (open and closed) carrying
// label, excluding pull requests. It walks every page: the result is
// meant to replace the stored set wholesale, so a partial listing is
// never returned — any page failure fails the call.
func (c *Client) ListIncidentIssues(
	ctx context.Context,
	repoName repo.FullName,
	label string,
) ([]incident.Incident, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues", repoName.Owner, repoName.Name)

	var out []incident.Incident
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		q := url.Values{}
		q.Set("labels", label)
		q.Set("state", "all")
		q.Set("sort", "created")
		q.Set("direction", "asc")
		q.Set("per_page", strconv.Itoa(issuesPerPage))
		q.Set("page", strconv.Itoa(page))

		var batch []rawIssue
		if _, err := c.rest(ctx, "GET", path, q, &batch); err != nil {
			return nil, fmt.Errorf("list incident issues: %w", err)
		}

		for _, is := range batch {
			if is.PullRequest != nil {
				continue
			}
			out = append(out, incident.Incident{
				Source:     incident.SourceGitHubIssue,
				Number:     is.Number,
				Title:      is.Title,
				OpenedAt:   is.CreatedAt.UTC(),
				ResolvedAt: timex.PtrUTC(is.ClosedAt),
			})
		}
		if len(batch) < issuesPerPage {
			return out, nil
		}
	}
}
