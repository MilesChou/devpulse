package fetching

import (
	"context"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/repo"
)

// CIProvider returns CI builds for a repo, incrementally.
// Implemented by internal/travis.
type CIProvider interface {
	// ListBuildsSince walks upstream builds newest-first and stops
	// paging once a page reaches `since`. The boundary page is
	// returned in full; callers dedupe via (repo_id, external_id).
	// When since.IsZero() it back-fills the full history (cold start).
	ListBuildsSince(ctx context.Context, repoName repo.FullName, since time.Time) ([]build.Build, error)
}
