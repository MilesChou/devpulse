package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

func newBuildFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <owner/name>",
		Short: "Fetch all CI builds for a repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuildFetch(cmd.Context(), args[0])
		},
	}
}

func runBuildFetch(ctx context.Context, repoArg string) error {
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

	written, err := d.orch.FetchAllBuilds(ctx, r)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout(), "Fetched %s builds: written=%d\n",
		name.String(), written,
	)
	return nil
}
