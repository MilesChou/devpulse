package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/metrics"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
)

func newMetricsCmd() *cobra.Command {
	var fromFlag, toFlag string

	cmd := &cobra.Command{
		Use:     "metrics <owner/name>",
		Short:   "Show engineering-efficiency metrics for a repo",
		Example: "  devpulse metrics MilesChou/devpulse --from 2026-05 --to 2026-06",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMetrics(cmd.Context(), args[0], fromFlag, toFlag)
		},
	}

	now := time.Now().UTC()
	defaultMonth := now.Format("2006-01")
	cmd.Flags().StringVar(&fromFlag, "from", defaultMonth, "start month (YYYY-MM)")
	cmd.Flags().StringVar(&toFlag, "to", "", "end month exclusive (YYYY-MM); defaults to one month after --from")

	return cmd
}

func runMetrics(ctx context.Context, repoArg, fromFlag, toFlag string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}

	w, err := metrics.ParseWindow(fromFlag, toFlag, time.Now())
	if err != nil {
		return err
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	r, err := d.repos.FindByFullName(ctx, "github", name)
	if err != nil {
		return fmt.Errorf("repo lookup: %w", err)
	}

	report, err := metrics.Compute(ctx, persistence.NewMetricsPersister(d.pers), metrics.Single(r), w, time.Now(), nil)
	if err != nil {
		return err
	}
	printMetrics(stdout(), report, w)
	return nil
}

func printMetrics(w io.Writer, r metrics.Report, win metrics.Window) {
	fmt.Fprintf(w, "Metrics for %s (%s)\n", r.Repo, win.Label())
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 40))

	fmt.Fprintf(w, "CI Failure Rate:        %.1f%% (%d/%d PR builds)\n",
		r.BuildFailure.Rate*100, r.BuildFailure.Failed, r.BuildFailure.Total)
	fmt.Fprintf(w, "Avg Builds per PR:      %.1f\n", r.AvgBuildsPerPR)
	fmt.Fprintf(w, "PR Lead Time:           avg %.1fh  p50 %.1fh  p90 %.1fh  (%d PRs)\n",
		r.PRLeadTime.AvgHours, r.PRLeadTime.P50Hours, r.PRLeadTime.P90Hours, r.PRLeadTime.Count)
	fmt.Fprintf(w, "Review Wait Time:       avg %.1fh (%d PRs)\n", r.ReviewWait.AvgHours, r.ReviewWait.Count)
	fmt.Fprintf(w, "PR Size Distribution:   %s\n", formatSizeDist(r.PRSizeDistribution))
	if b := r.BuildDuration; b.Count > 0 {
		fmt.Fprintf(w, "Build Duration:         p50 %.0fs  avg %.0fs  p90 %.0fs  (%d builds)\n",
			b.P50Seconds, b.AvgSeconds, b.P90Seconds, b.Count)
	}

	if len(r.DailyBuildDuration) > 0 {
		fmt.Fprintf(w, "\nDaily Build Duration (median seconds):\n")
		for _, d := range r.DailyBuildDuration {
			fmt.Fprintf(w, "  %s: %.0fs (%d builds)\n", d.Day, d.P50Seconds, d.Count)
		}
	}

	printDORA(w, r.DORA, r.Repo)
}

// printDORA renders the DORA section. A nil section means the default
// branch is unknown, and deployments are merges into it, so the output
// says how to fix that instead of guessing.
func printDORA(w io.Writer, d *metrics.DORA, repoName string) {
	fmt.Fprintf(w, "\nDORA (deployment = PR merged into default branch)\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 40))

	if d == nil {
		fmt.Fprintf(w, "Default branch unknown; run `devpulse repo refresh %s` first.\n", repoName)
		return
	}

	fmt.Fprintf(w, "Deployment Frequency:   %d deploys into %s  (%.2f/week, %d deploy days)\n",
		d.Deployments, d.DefaultBranch, d.PerWeek, d.DeployDays)
	fmt.Fprintf(w, "Lead Time for Changes:  %s\n", formatSummary(d.LeadTime, "deploys"))

	cfr := "n/a (no deploys)"
	if d.ChangeFailureRate != nil {
		cfr = fmt.Sprintf("%.1f%% (%d/%d)", *d.ChangeFailureRate*100, d.Reverts+d.Hotfixes, d.Deployments)
	}
	fmt.Fprintf(w, "Change Failure Rate:    %s  reverts=%d hotfixes=%d (label %q)\n",
		cfr, d.Reverts, d.Hotfixes, d.HotfixLabel)
	fmt.Fprintf(w, "Recovery Time:          %s  from reverts=%d incidents=%d (label %q)\n",
		formatSummary(d.Recovery, "samples"), d.RecoveryFromReverts, d.RecoveryFromIncidents, d.IncidentLabel)
}

// formatSummary renders an hours summary, or "(no data)" for an empty
// sample so a zero average is never mistaken for an instant result.
func formatSummary(s metrics.HoursSummary, unit string) string {
	if s.Count == 0 {
		return "(no data)"
	}
	return fmt.Sprintf("avg %.1fh  p50 %.1fh  p90 %.1fh  (%d %s)", s.AvgHours, s.P50Hours, s.P90Hours, s.Count, unit)
}

// formatSizeDist prints only the buckets that have PRs, so an empty
// window reads "(no data)" rather than a row of zeros.
func formatSizeDist(dist []metrics.SizeBucketCount) string {
	var parts []string
	for _, b := range dist {
		if b.Count > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", b.Bucket, b.Count))
		}
	}
	if len(parts) == 0 {
		return "(no data)"
	}
	return strings.Join(parts, "  ")
}
