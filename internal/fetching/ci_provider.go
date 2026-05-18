package fetching

import (
	"context"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/repo"
)

// CIProvider returns CI builds for a repo within a calendar month.
// Implemented by internal/travis.
type CIProvider interface {
	ListBuildsInMonth(ctx context.Context, repoName repo.FullName, month MonthRange) ([]build.Build, error)
}
