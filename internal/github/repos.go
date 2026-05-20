package github

import (
	"context"
	"fmt"

	"github.com/mileschou/devpulse/internal/repo"
)

// rawRepo is the slim subset of GET /repos/:owner/:name we care about.
// GitHub returns many more fields; unknown keys are tolerated by the
// JSON decoder.
type rawRepo struct {
	Description   *string `json:"description"`
	DefaultBranch string  `json:"default_branch"`
	Disabled      bool    `json:"disabled"`
}

// GetRepo fetches repo metadata (description, default branch, disabled
// flag) from GitHub. The returned Repo has zero ID — the caller is
// expected to overlay it on an existing aggregate.
func (c *Client) GetRepo(ctx context.Context, name repo.FullName) (repo.Repo, error) {
	path := fmt.Sprintf("/repos/%s/%s", name.Owner, name.Name)

	var raw rawRepo
	if _, err := c.rest(ctx, "GET", path, nil, &raw); err != nil {
		return repo.Repo{}, err
	}

	return repo.Repo{
		Name:          name,
		Description:   raw.Description,
		DefaultBranch: raw.DefaultBranch,
		Disabled:      raw.Disabled,
	}, nil
}
