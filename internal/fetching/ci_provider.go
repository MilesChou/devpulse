package fetching

import (
	"context"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/repo"
)

// CIProvider returns CI builds for a repo.
// Implemented by internal/travis.
type CIProvider interface {
	ListAllBuilds(ctx context.Context, repoName repo.FullName) ([]build.Build, error)
}
