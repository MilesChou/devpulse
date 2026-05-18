// Package cli wires together cobra commands for the devpulse binary.
//
// The CLI follows a noun-on-verb layout (in the style of gh, jira-cli):
//
//	devpulse repo add ...
//	devpulse build fetch ...
//	devpulse pr fetch ...
//	devpulse pr enrich ...
//	devpulse migrate up | down | status
//	devpulse worker
//	devpulse serve
//
// Each group lives in cmd_<group>.go; each leaf action lives in
// cmd_<group>_<verb>.go.
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
		newRepoCmd(),
		newBuildCmd(),
		newPRCmd(),
		newMigrateCmd(),
		newWorkerCmd(),
		newServeCmd(),
	)
	return root
}
