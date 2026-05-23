package cli

import (
	"context"
	"fmt"

	"github.com/mileschou/devpulse/internal/repo"
)

// syncOneRepo runs PR sync (with enrichment) then CI build sync for a
// single repo, in that order. Identical surface to `devpulse repo sync`
// but takes a pre-resolved Repo so the top-level `devpulse sync` can
// iterate over the store without re-ensuring each row.
//
// Progress messages mirror the single-repo command so a multi-repo run
// reads as a concatenation of individual syncs. The PR step always
// prints the written count (even on failure) because writers upsert
// page-by-page and partial progress is real progress; that count would
// be lost if we only printed on success.
func syncOneRepo(ctx context.Context, d *deps, r repo.Repo) error {
	prsWritten, prsErr := d.orch.FetchAllPullRequestsWithEnrichment(ctx, r)
	fmt.Fprintf(stdout(), "Synced %s pull requests: written=%d\n", r.Name.String(), prsWritten)
	if prsErr != nil {
		return fmt.Errorf("sync pull requests: %w", prsErr)
	}

	buildsWritten, err := d.orch.FetchAllBuilds(ctx, r)
	if err != nil {
		return fmt.Errorf("sync ci builds: %w", err)
	}
	fmt.Fprintf(stdout(), "Synced %s ci builds: written=%d\n", r.Name.String(), buildsWritten)
	return nil
}
