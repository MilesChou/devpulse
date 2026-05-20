package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

func newRepoRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh <owner/name>",
		Short: "Re-fetch GitHub metadata (description, default_branch, disabled) for a tracked repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoRefresh(cmd.Context(), args[0])
		},
	}
}

func runRepoRefresh(ctx context.Context, repoArg string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	existing, err := d.repos.FindByFullName(ctx, "github", name)
	if err != nil {
		return fmt.Errorf("find repo: %w", err)
	}

	meta, err := d.vcs.GetRepo(ctx, name)
	if err != nil {
		return fmt.Errorf("fetch github metadata: %w", err)
	}

	if err := d.repos.UpdateMetadata(ctx, existing.ID, meta); err != nil {
		return fmt.Errorf("update metadata: %w", err)
	}

	meta.ID = existing.ID
	meta.Name = existing.Name
	printRepoSummary(stdout(), meta)
	return nil
}
