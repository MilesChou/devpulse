package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
)

// reviewsQuery walks the Reviews connection with the GitHub maximum
// page size (first: 100). The $after cursor is null on the first call
// and the previous page's endCursor afterwards.
const reviewsQuery = `
query($owner: String!, $name: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviews(first: 100, after: $after) {
        nodes {
          state
          submittedAt
          author { login }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}
`

type reviewsResponse struct {
	Repository struct {
		PullRequest struct {
			Reviews struct {
				Nodes    []reviewNode `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"reviews"`
		} `json:"pullRequest"`
	} `json:"repository"`
}

type reviewNode struct {
	State       string     `json:"state"`
	SubmittedAt *time.Time `json:"submittedAt"`
	Author      *struct {
		Login string `json:"login"`
	} `json:"author"`
}

// ListReviews fetches every review on the PR via GraphQL, paging through
// the entire connection. Truncation at the GitHub 100-node page size is
// transparently handled — callers always see the full set.
//
// Two categories of node are silently dropped: pending reviews (no
// submittedAt) and ghost-author reviews (no login). Neither contributes
// usable data to the latency model, but we tolerate them without erroring.
//
// reviewsHardCap is an upper bound on pages walked, present only to
// prevent an infinite loop if GitHub ever returns hasNextPage=true with
// no progress. With reviewsPageSize=100 it corresponds to 10,000 reviews
// per PR — a value no real PR comes close to.
func (c *Client) ListReviews(
	ctx context.Context,
	repoName repo.FullName,
	number int,
) ([]pullrequest.Review, error) {
	const reviewsHardCap = 100 // pages

	var (
		out    []pullrequest.Review
		cursor string
	)

	for range reviewsHardCap {
		if err := ctx.Err(); err != nil {
			return out, err
		}

		vars := map[string]any{
			"owner":  repoName.Owner,
			"name":   repoName.Name,
			"number": number,
		}
		if cursor != "" {
			vars["after"] = cursor
		} else {
			vars["after"] = nil
		}

		var resp reviewsResponse
		if err := c.graphql(ctx, reviewsQuery, vars, &resp); err != nil {
			return nil, err
		}

		reviews := resp.Repository.PullRequest.Reviews
		for _, n := range reviews.Nodes {
			if n.SubmittedAt == nil || n.Author == nil || n.Author.Login == "" {
				continue
			}
			out = append(out, pullrequest.Review{
				ReviewerAccount: n.Author.Login,
				State:           parseReviewState(n.State),
				SubmittedAt:     n.SubmittedAt.UTC(),
			})
		}

		if !reviews.PageInfo.HasNextPage || reviews.PageInfo.EndCursor == "" {
			return out, nil
		}
		cursor = reviews.PageInfo.EndCursor
	}

	return out, fmt.Errorf("github: ListReviews exceeded %d pages for %s#%d (possible upstream loop)",
		reviewsHardCap, repoName.String(), number)
}

// parseReviewState maps GitHub GraphQL review enums to the domain.
//
//	APPROVED          -> Approved
//	CHANGES_REQUESTED -> ChangesRequested
//	COMMENTED         -> Commented
//	DISMISSED         -> Dismissed
//	PENDING / others  -> Unknown (caller usually filters earlier on
//	                    submittedAt == nil)
func parseReviewState(s string) pullrequest.ReviewState {
	switch strings.ToUpper(s) {
	case "APPROVED":
		return pullrequest.ReviewStateApproved
	case "CHANGES_REQUESTED":
		return pullrequest.ReviewStateChangesRequested
	case "COMMENTED":
		return pullrequest.ReviewStateCommented
	case "DISMISSED":
		return pullrequest.ReviewStateDismissed
	default:
		return pullrequest.ReviewStateUnknown
	}
}
