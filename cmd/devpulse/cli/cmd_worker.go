package cli

import (
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/jobs"
)

func newWorkerCmd() *cobra.Command {
	var pollEvery time.Duration
	var leaseFor time.Duration

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run the DevPulse job worker (long-running, ^C to stop)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			d, err := buildDeps(ctx)
			if err != nil {
				return err
			}
			defer d.close(ctx)

			q := jobs.NewQueue(d.pers)
			handlers := map[string]jobs.HandlerFunc{
				jobs.KindEnrichPullRequest: jobs.NewEnrichPullRequestHandler(d.orch, d.repos),
			}
			w := jobs.NewWorker(q, handlers, pollEvery, leaseFor, nil)
			fmt.Fprintln(stdout(), "worker started; press Ctrl-C to stop")
			if err := w.Run(ctx); err != nil && err != ctx.Err() {
				return err
			}
			fmt.Fprintln(stdout(), "worker stopped")
			return nil
		},
	}
	cmd.Flags().DurationVar(&pollEvery, "poll", 5*time.Second, "Poll interval between empty ticks")
	cmd.Flags().DurationVar(&leaseFor, "lease", 60*time.Second, "Lease duration before a stuck job is requeued")
	return cmd
}

