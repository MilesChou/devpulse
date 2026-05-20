package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

func newPRFetchAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch-all <owner/name>",
		Short: "Fetch all historical PRs with enrichment for a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPRFetchAll(cmd.Context(), args[0])
		},
	}
}

func runPRFetchAll(ctx context.Context, repoArg string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
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

	written, err := d.orch.FetchAllPullRequestsWithEnrichment(ctx, r)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout(), "Fetched all %s PRs: written=%d\n", name.String(), written)
	return nil
}
