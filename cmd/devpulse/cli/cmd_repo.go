package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

// newRepoCmd is the `devpulse repo` command group. Concrete actions
// (add / list / remove) live in cmd_repo_*.go.
func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage tracked repositories",
	}
	cmd.AddCommand(newRepoAddCmd(), newRepoRefreshCmd())
	return cmd
}

func printRepoSummary(w io.Writer, r repo.Repo) {
	fmt.Fprintf(w, "%s (id=%s, default_branch=%q, disabled=%v, description=%s)\n",
		r.Name.String(), r.ID, r.DefaultBranch, r.Disabled, formatDescription(r.Description),
	)
}

func formatDescription(p *string) string {
	if p == nil {
		return "(none)"
	}
	return fmt.Sprintf("%q", *p)
}
