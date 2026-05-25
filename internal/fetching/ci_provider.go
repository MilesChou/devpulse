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
	// ListBuildsSince walks upstream builds in newest-first order and
	// stops paging once it sees a build whose started_at is at or before
	// `since`. The page containing that boundary build is returned in
	// full — the slice may include a few rows with started_at <= since
	// at the tail, which the caller's dedupe (unique on
	// (repo_id, external_id)) silently filters out. This widens the
	// effective overlap window enough to cover retried builds whose
	// started_at lands slightly behind the watermark.
	//
	// When since.IsZero() the implementation MUST back-fill the full
	// upstream history (cold-start semantics). This is how a fresh DB
	// gets populated on the first sync.
	ListBuildsSince(ctx context.Context, repoName repo.FullName, since time.Time) ([]build.Build, error)
}
