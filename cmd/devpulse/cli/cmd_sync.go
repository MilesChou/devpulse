package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/repo"
)

// newSyncCmd is the top-level `devpulse sync` command. It walks every
// tracked repo in the store and runs the same PR + CI build sync as
// `devpulse repo sync` against each one, sequentially.
//
// Per-repo failures are recorded and reported in a final summary; they
// do not abort the loop, so one bad repo does not block the rest of the
// batch from making progress. Disabled repos are skipped — those are
// repos GitHub has archived/disabled, where syncing wastes API quota and
// almost always errors. Sequential (not parallel) by design: GitHub and
// Travis both rate-limit per-token, so parallelism would bunch the burn
// without buying meaningful throughput.
//
// Exit code is non-zero when any repo failed, so this is safe to drive
// from cron / CI.
func newSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync every tracked repo (PRs with enrichment, then CI builds)",
		Long: "Runs the equivalent of `devpulse repo sync` against every " +
			"repository in the store, sequentially. Disabled repos are " +
			"skipped. Per-repo failures are aggregated into a final summary " +
			"but do not stop the loop. Requires GITHUB_TOKEN and TRAVIS_TOKEN.",
		Example: "  devpulse sync",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context())
		},
	}
}

// syncFailure records a per-repo error so the summary can list every
// failure even when the loop continues past the first one.
type syncFailure struct {
	repo repo.FullName
	err  error
}

func runSync(ctx context.Context) error {
	// Fail-fast on missing tokens, before opening the DB or starting the
	// OTel exporter — same contract as `repo sync`. config.Load does not
	// require the tokens, so commands like `migrate` and `repo add` keep
	// working without them.
	if strings.TrimSpace(os.Getenv("GITHUB_TOKEN")) == "" {
		return errors.New("sync: GITHUB_TOKEN is required")
	}
	if strings.TrimSpace(os.Getenv("TRAVIS_TOKEN")) == "" {
		return errors.New("sync: TRAVIS_TOKEN is required")
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	repos, err := d.repos.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}

	w := stdout()
	if len(repos) == 0 {
		fmt.Fprintln(w, "sync: no repos in store; use `devpulse repo add` first")
		return nil
	}

	var (
		synced   int
		skipped  int
		failures []syncFailure
	)

	for _, r := range repos {
		if r.Disabled {
			fmt.Fprintf(w, "skipped %s (disabled)\n", r.Name.String())
			skipped++
			continue
		}

		if err := syncOneRepo(ctx, d, r); err != nil {
			// Print the per-repo failure inline so it lands next to that
			// repo's own progress lines; the summary at the end lists them
			// all together again for grep-ability.
			fmt.Fprintf(w, "failed %s: %v\n", r.Name.String(), err)
			failures = append(failures, syncFailure{repo: r.Name, err: err})
			continue
		}
		synced++
	}

	fmt.Fprintf(w, "\nsync: synced=%d skipped=%d failed=%d\n",
		synced, skipped, len(failures))

	if len(failures) > 0 {
		fmt.Fprintln(w, "failures:")
		for _, f := range failures {
			fmt.Fprintf(w, "  %s: %v\n", f.repo.String(), f.err)
		}
		return fmt.Errorf("sync: %d repo(s) failed", len(failures))
	}
	return nil
}
