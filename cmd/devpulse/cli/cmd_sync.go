package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

// newSyncCmd is the top-level `devpulse sync` command. It pulls all PRs
// (including reviews and enrichment) then all CI builds for a repository,
// in that order. Either step failing aborts the run; partial progress
// from earlier steps is still committed because each writer upserts.
func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <owner/name>",
		Short: "Sync PRs (with enrichment) then CI builds for a repo",
		Long: "Pulls all pull requests (with reviews and enrichment) from GitHub, " +
			"then all CI builds from Travis, for the given repository. " +
			"Requires GITHUB_TOKEN and TRAVIS_TOKEN. PRs are synced first; if " +
			"that step fails the build step is skipped.",
		Example: "  devpulse sync MilesChou/devpulse",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context(), args[0])
		},
	}
}

func runSync(ctx context.Context, repoArg string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}

	// Validate required tokens before buildDeps to truly fail-fast: no DB
	// open, no OTel exporter started, no HTTP client built. config.Load
	// does not require them so commands like migrate/repo add still work
	// without them; sync enforces both at the entry point.
	if strings.TrimSpace(os.Getenv("GITHUB_TOKEN")) == "" {
		return errors.New("sync: GITHUB_TOKEN is required")
	}
	if strings.TrimSpace(os.Getenv("TRAVIS_TOKEN")) == "" {
		return errors.New("sync: TRAVIS_TOKEN is required")
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	r, err := d.repos.EnsureID(ctx, "github", name)
	if err != nil {
		return fmt.Errorf("ensure repo: %w", err)
	}

	prsWritten, prsErr := d.orch.FetchAllPullRequestsWithEnrichment(ctx, r)
	// FetchAllPullRequestsWithEnrichment returns a partial count even on
	// failure (it upserts page-by-page), so print what landed before
	// bubbling the error up.
	fmt.Fprintf(stdout(), "Synced %s pull requests: written=%d\n", name.String(), prsWritten)
	if prsErr != nil {
		return fmt.Errorf("sync pull requests: %w", prsErr)
	}

	buildsWritten, err := d.orch.FetchAllBuilds(ctx, r)
	if err != nil {
		return fmt.Errorf("sync ci builds: %w", err)
	}
	fmt.Fprintf(stdout(), "Synced %s ci builds: written=%d\n", name.String(), buildsWritten)

	return nil
}
