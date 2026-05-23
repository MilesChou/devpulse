package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

func newPRSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <owner/name> <number>",
		Short: "Re-fetch detail and reviews for one PR and write the enrichment patch",
		Long: "Refreshes a single pull request that is already in the store: " +
			"re-fetches detail and review data from GitHub and writes the " +
			"enrichment patch. Use `devpulse sync` to back-fill PRs that are " +
			"not yet in the store.",
		Example: "  devpulse pr sync MilesChou/devpulse 42",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPRSync(cmd.Context(), args[0], args[1])
		},
	}
}

func runPRSync(ctx context.Context, repoArg, numArg string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}
	num, err := strconv.Atoi(numArg)
	if err != nil || num <= 0 {
		return fmt.Errorf("invalid PR number: %q", numArg)
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

	found, err := d.orch.EnrichOnePullRequestByNumber(ctx, r, num)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("PR #%d not in store; run `devpulse sync` first", num)
	}
	fmt.Fprintf(stdout(), "Synced %s#%d\n", name.String(), num)
	return nil
}
