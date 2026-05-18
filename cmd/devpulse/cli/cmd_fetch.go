package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/repo"
)

func newFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <owner/name> <YYYY-MM>",
		Short: "Fetch CI builds and PRs for a repo in a given month",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetch(cmd.Context(), args[0], args[1])
		},
	}
}

func runFetch(ctx context.Context, repoArg, monthArg string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}
	month, err := fetching.ParseMonthRange(monthArg)
	if err != nil {
		return fmt.Errorf("invalid month: %w", err)
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

	outcome := d.orch.Fetch(ctx, r, month)
	if outcome.Error != nil {
		return outcome.Error
	}
	fmt.Fprintf(stdout(), "Fetched %s for %s: builds=%d pulls=%d\n",
		outcome.RepoFullName, month.String(),
		outcome.BuildsWritten, outcome.PullRequestsWritten,
	)
	return nil
}
