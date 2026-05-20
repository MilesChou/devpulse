package cli

import "github.com/spf13/cobra"

// newPRCmd is the `devpulse pr` command group covering pull-request
// operations. Concrete actions live in cmd_pr_*.go.
func newPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Pull request operations",
	}
	cmd.AddCommand(newPRFetchCmd(), newPRFetchAllCmd(), newPREnrichCmd())
	return cmd
}
