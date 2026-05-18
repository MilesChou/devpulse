// Package cli wires together cobra commands for the devpulse binary.
//
// Convention follows ory/hydra: one file per subcommand, named
// cmd_<verb>_<noun>.go.
package cli

import (
	"context"

	"github.com/spf13/cobra"
)

// Execute runs the root command, returning the first error from any
// subcommand. Errors are printed by the cobra harness in main.go.
func Execute(ctx context.Context) error {
	return NewRootCmd().ExecuteContext(ctx)
}

// NewRootCmd builds the root cobra command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "devpulse",
		Short:         "DevPulse — CI and PR engineering-efficiency observability",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newFetchCmd(),
		newEnrichPRCmd(),
		newRepoAddCmd(),
		newMigrateCmd(),
		newWorkerCmd(),
		newServeCmd(),
	)
	return root
}
