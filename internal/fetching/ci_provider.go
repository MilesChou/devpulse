package fetching

import (
	"context"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/repo"
)

// CIProvider returns CI builds for a repo, incrementally.
// Implemented by internal/travis and internal/github.
type CIProvider interface {
	// Name identifies the provider ("travis", "github-actions").
	// It namespaces both the per-provider sync watermark and the
	// builds rows themselves — external IDs are only unique within
	// one provider, so the DB dedupe key is
	// (repo_id, ci_provider, external_id).
	Name() string

	// ListBuildsSince walks upstream builds newest-first and stops
	// paging once a page reaches `since`. The boundary page is
	// returned in full; callers dedupe via
	// (repo_id, ci_provider, external_id).
	// When since.IsZero() it back-fills the full history (cold start).
	ListBuildsSince(ctx context.Context, repoName repo.FullName, since time.Time) ([]build.Build, error)
}
