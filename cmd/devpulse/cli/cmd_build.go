package cli

import "github.com/spf13/cobra"

// newBuildCmd is the `devpulse build` command group covering CI build
// data. Concrete actions live in cmd_build_*.go.
func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "CI build operations",
	}
	cmd.AddCommand(newBuildFetchCmd())
	return cmd
}
