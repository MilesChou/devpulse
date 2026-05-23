package travis

import (
	"context"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/repo"
)

// Provider adapts the Travis Client to fetching.CIProvider.
type Provider struct{ client *Client }

func NewProvider(c *Client) *Provider { return &Provider{client: c} }

// Compile-time assertion.
var _ fetching.CIProvider = (*Provider)(nil)

func (p *Provider) ListAllBuilds(
	ctx context.Context,
	repoName repo.FullName,
) ([]build.Build, error) {
	return p.client.ListAllBuilds(ctx, repoName.String())
}
