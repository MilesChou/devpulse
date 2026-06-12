package github

import (
	"context"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/repo"
)

// ActionsProvider adapts the GitHub Client to fetching.CIProvider
// using the GitHub Actions workflow runs API.
type ActionsProvider struct{ client *Client }

func NewActionsProvider(c *Client) *ActionsProvider { return &ActionsProvider{client: c} }

var _ fetching.CIProvider = (*ActionsProvider)(nil)

func (p *ActionsProvider) ListBuildsSince(
	ctx context.Context,
	repoName repo.FullName,
	since time.Time,
) ([]build.Build, error) {
	return p.client.ListActionRuns(ctx, repoName.Owner, repoName.Name, since)
}
