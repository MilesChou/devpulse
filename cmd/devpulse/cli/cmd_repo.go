package cli

import "github.com/spf13/cobra"

// newRepoCmd is the `devpulse repo` command group. Concrete actions
// (add / list / remove) live in cmd_repo_*.go.
func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage tracked repositories",
	}
	cmd.AddCommand(newRepoAddCmd())
	return cmd
}
