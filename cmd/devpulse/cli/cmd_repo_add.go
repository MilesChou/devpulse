package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

func newRepoAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "repo-add <owner/name>",
		Short: "Register a repository in the DevPulse store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoAdd(cmd.Context(), args[0])
		},
	}
}

func runRepoAdd(ctx context.Context, repoArg string) error {
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
	fmt.Fprintf(stdout(), "%s (id=%s)\n", r.Name.String(), r.ID)
	return nil
}

