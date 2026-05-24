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

func (p *Provider) GetPullRequest(
	ctx context.Context,
	repoID string,
	repoName repo.FullName,
	number int,
) (pullrequest.PullRequest, error) {
	return p.client.GetPullRequest(ctx, repoID, repoName, number)
}

func (p *Provider) GetLatestPRNumber(
	ctx context.Context,
	repoName repo.FullName,
) (int, error) {
	return p.client.GetLatestPRNumber(ctx, repoName)
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
