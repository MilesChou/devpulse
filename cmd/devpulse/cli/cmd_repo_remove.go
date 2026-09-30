package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/repoadmin"
)

func newRepoRemoveCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <owner/name>",
		Short: "Stop tracking a repository and delete its synced data",
		Long: "Deletes the repo and every pull request, review, build and incident " +
			"synced for it. Registering it again later re-syncs from scratch. " +
			"Requires --yes, since the deletion cannot be undone.",
		Example: "  devpulse repo remove acme/legacy --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoRemove(cmd.Context(), args[0], yes)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deleting the repo and its synced data")
	return cmd
}

func runRepoRemove(ctx context.Context, repoArg string, yes bool) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}
	if !yes {
		return fmt.Errorf("refusing to delete %s and its synced data without --yes", name)
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	if err := d.admin.Remove(ctx, name); err != nil {
		if errors.Is(err, repoadmin.ErrNotFound) {
			return fmt.Errorf("repo %s is not registered", name)
		}
		return fmt.Errorf("remove %s: %w", name, err)
	}
	fmt.Fprintf(stdout(), "Removed %s\n", name)
	return nil
}
