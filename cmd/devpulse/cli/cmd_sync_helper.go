package cli

import (
	"context"
	"fmt"

	"github.com/mileschou/devpulse/internal/repo"
)

// syncOneRepo refreshes stored open PRs, runs PR sync (by ascending
// number), CI build sync, and incident sync for a single repo, in that
// order. The open-PR refresh runs first so PRs the backfill writes in
// this run are not fetched twice. Incident sync failures are reported
// but do not fail the repo, mirroring how a failing CI provider is
// skipped. Identical surface to `devpulse repo sync`
// but takes a pre-resolved Repo so the top-level `devpulse sync` can
// iterate over the store without re-ensuring each row.
//
// Progress messages mirror the single-repo command so a multi-repo run
// reads as a concatenation of individual syncs. The PR step always
// prints the written count (even on failure) because each PR is written
// atomically and partial progress is real progress; that count would be
// lost if we only printed on success.
func syncOneRepo(ctx context.Context, d *deps, r repo.Repo) error {
	refreshed, err := d.orch.RefreshOpenPullRequests(ctx, r)
	if err != nil {
		return fmt.Errorf("refresh open pull requests: %w", err)
	}
	fmt.Fprintf(stdout(), "Refreshed %s open pull requests: %d\n", r.Name.String(), refreshed)

	prsWritten, prsErr := d.orch.BackfillPullRequestsByNumber(ctx, r)
	fmt.Fprintf(stdout(), "Synced %s pull requests: written=%d\n", r.Name.String(), prsWritten)
	if prsErr != nil {
		return fmt.Errorf("sync pull requests: %w", prsErr)
	}

	buildsWritten, err := d.orch.FetchAllBuilds(ctx, r)
	if err != nil {
		return fmt.Errorf("sync ci builds: %w", err)
	}
	fmt.Fprintf(stdout(), "Synced %s ci builds: written=%d\n", r.Name.String(), buildsWritten)

	incidents, err := d.orch.SyncIncidents(ctx, r)
	if err != nil {
		fmt.Fprintf(stdout(), "Warning: sync %s incidents (label %q): %v\n", r.Name.String(), r.IncidentLabel, err)
		return nil
	}
	fmt.Fprintf(stdout(), "Synced %s incidents (label %q): %d\n", r.Name.String(), r.IncidentLabel, incidents)
	return nil
}
