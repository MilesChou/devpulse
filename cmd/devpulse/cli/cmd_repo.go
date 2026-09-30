package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

// newRepoCmd is the `devpulse repo` command group. Concrete actions
// (add / refresh / sync) live in cmd_repo_*.go.
func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage tracked repositories",
	}
	cmd.AddCommand(newRepoAddCmd(), newRepoRemoveCmd(), newRepoRefreshCmd(), newRepoSyncCmd(), newRepoConfigCmd())
	return cmd
}

// registerRepo ensures a repo exists in the store and best-effort fetches
// GitHub metadata. Used by both `repo add` and `init`.
func registerRepo(ctx context.Context, w io.Writer, d *deps, name repo.FullName) (repo.Repo, error) {
	reg, err := d.admin.Register(ctx, name)
	if err != nil {
		return reg.Repo, err
	}
	if reg.MetadataErr != nil {
		fmt.Fprintf(w, "warn: %s: %v\n", name, reg.MetadataErr)
	}
	return reg.Repo, nil
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
