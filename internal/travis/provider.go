package travis

import (
	"context"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/repo"
)

// Provider adapts the Travis Client to fetching.CIProvider.
type Provider struct{ client *Client }

func NewProvider(c *Client) *Provider { return &Provider{client: c} }

// Compile-time assertion.
var _ fetching.CIProvider = (*Provider)(nil)

// Name implements fetching.CIProvider.
func (p *Provider) Name() string { return "travis" }

func (p *Provider) ListBuildsSince(
	ctx context.Context,
	repoName repo.FullName,
	since time.Time,
) ([]build.Build, error) {
	return p.client.ListBuildsSince(ctx, repoName.String(), since)
}
