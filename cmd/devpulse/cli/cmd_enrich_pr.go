package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

func newEnrichPRCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enrich-pr <owner/name> <number>",
		Short: "Re-fetch detail and reviews for one PR and write the enrichment patch",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnrichPR(cmd.Context(), args[0], args[1])
		},
	}
}

func runEnrichPR(ctx context.Context, repoArg, numArg string) error {
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
		return fmt.Errorf("PR #%d not in store; run `devpulse fetch` first", num)
	}
	fmt.Fprintf(stdout(), "Enriched %s#%d\n", name.String(), num)
	return nil
}

