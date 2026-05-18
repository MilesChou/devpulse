package github

import (
	"context"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
)

const reviewsQuery = `
query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviews(first: 100) {
        nodes {
          state
          submittedAt
          author { login }
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
				Nodes []reviewNode `json:"nodes"`
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

// ListReviews fetches every review on the PR via GraphQL.
//
// Two categories of node are silently dropped: pending reviews (no
// submittedAt) and ghost-author reviews (no login). Neither contributes
// usable data to the latency model, but we tolerate them without erroring.
func (c *Client) ListReviews(
	ctx context.Context,
	repoName repo.FullName,
	number int,
) ([]pullrequest.Review, error) {
	vars := map[string]any{
		"owner":  repoName.Owner,
		"name":   repoName.Name,
		"number": number,
	}

	var resp reviewsResponse
	if err := c.graphql(ctx, reviewsQuery, vars, &resp); err != nil {
		return nil, err
	}

	out := make([]pullrequest.Review, 0, len(resp.Repository.PullRequest.Reviews.Nodes))
	for _, n := range resp.Repository.PullRequest.Reviews.Nodes {
		if n.SubmittedAt == nil || n.Author == nil || n.Author.Login == "" {
			continue
		}
		out = append(out, pullrequest.Review{
			ReviewerAccount: n.Author.Login,
			State:           parseReviewState(n.State),
			SubmittedAt:     n.SubmittedAt.UTC(),
		})
	}
	return out, nil
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

