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

// newRepoSyncCmd is the `devpulse repo sync` command. It pulls all PRs
// (including reviews and enrichment) then all CI builds for a repository,
// in that order. Either step failing aborts the run; partial progress
// from earlier steps is still committed because each writer upserts.
func newRepoSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <owner/name>",
		Short: "Sync PRs (with enrichment) then CI builds for a repo",
		Long: "Pulls all pull requests (with reviews and enrichment) from GitHub, " +
			"then all CI builds from GitHub Actions, for the given repository. " +
			"Requires GITHUB_TOKEN. PRs are synced first; if " +
			"that step fails the build step is skipped.",
		Example: "  devpulse repo sync MilesChou/devpulse",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoSync(cmd.Context(), args[0])
		},
	}
}

func runRepoSync(ctx context.Context, repoArg string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}

	// Validate required tokens before buildDeps to truly fail-fast: no DB
	// open, no OTel exporter started, no HTTP client built. config.Load
	// does not require them so commands like migrate/repo add still work
	// without them; repo sync enforces the token at the entry point.
	if strings.TrimSpace(os.Getenv("GITHUB_TOKEN")) == "" {
		return errors.New("repo sync: GITHUB_TOKEN is required")
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

	return syncOneRepo(ctx, d, r)
}
