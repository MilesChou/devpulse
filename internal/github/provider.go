package github

import (
	"context"

	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// Provider adapts the GitHub Client to fetching.VCSProvider.
type Provider struct {
	client *Client
}

// NewProvider wraps a Client.
func NewProvider(c *Client) *Provider { return &Provider{client: c} }

// Compile-time assertion: Provider must satisfy fetching.VCSProvider.
var _ fetching.VCSProvider = (*Provider)(nil)

func (p *Provider) ListPullRequestsInMonth(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
	month fetching.MonthRange,
) ([]pullrequest.PullRequest, error) {
	return p.client.ListPullRequestsInMonth(ctx, repoID, repoName, month.Start, month.End)
}

func (p *Provider) ListAllPullRequests(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
) ([]pullrequest.PullRequest, error) {
	return p.client.ListAllPullRequests(ctx, repoID, repoName)
}

func (p *Provider) ListAllPullRequestsPageFunc(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
	fn func(page []pullrequest.PullRequest) error,
) error {
	return p.client.ListAllPullRequestsPageFunc(ctx, repoID, repoName, fn)
}

func (p *Provider) GetPullRequest(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
	number int,
) (pullrequest.PullRequest, error) {
	return p.client.GetPullRequest(ctx, repoID, repoName, number)
}

func (p *Provider) ListReviews(
	ctx context.Context,
	repoName repo.FullName,
	number int,
) ([]pullrequest.Review, error) {
	return p.client.ListReviews(ctx, repoName, number)
}

func (p *Provider) GetCommitAuthorAccountsBulk(
	ctx context.Context,
	repoName repo.FullName,
	shas []commitsha.SHA,
) (map[commitsha.SHA]*string, error) {
	return p.client.GetCommitAuthorAccountsBulk(ctx, repoName, shas)
}

func (p *Provider) GetRepo(ctx context.Context, repoName repo.FullName) (repo.Repo, error) {
	return p.client.GetRepo(ctx, repoName)
}
