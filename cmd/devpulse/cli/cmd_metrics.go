package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/dora"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/statx"
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

	from, err := parseMonth(fromFlag)
	if err != nil {
		return fmt.Errorf("invalid --from: %w", err)
	}

	var to time.Time
	if toFlag == "" {
		to = from.AddDate(0, 1, 0)
	} else {
		to, err = parseMonth(toFlag)
		if err != nil {
			return fmt.Errorf("invalid --to: %w", err)
		}
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

	metrics := persistence.NewMetricsPersister(d.pers)
	return printMetrics(ctx, metrics, r, from, to)
}

func printMetrics(ctx context.Context, m *persistence.MetricsPersister, r repo.Repo, from, to time.Time) error {
	w := stdout()
	repoID, repoName := r.ID, r.Name.String()

	// Single-month windows render as "2026-01"; anything wider shows
	// the inclusive month range so a multi-month aggregate is not
	// mistaken for one month's numbers.
	label := from.Format("2006-01")
	if lastMonth := to.AddDate(0, -1, 0); lastMonth.After(from) {
		label = fmt.Sprintf("%s ~ %s", from.Format("2006-01"), lastMonth.Format("2006-01"))
	}

	fmt.Fprintf(w, "Metrics for %s (%s)\n", repoName, label)
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 40))

	total, failed, rate, err := m.BuildFailureRate(ctx, repoID, from, to)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "CI Failure Rate:        %.1f%% (%d/%d PR builds)\n", rate*100, failed, total)

	avg, err := m.AverageBuildsPerPR(ctx, repoID, from, to)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Avg Builds per PR:      %.1f\n", avg)

	prCount, avgH, p50H, p90H, err := m.PRLeadTime(ctx, repoID, from, to)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "PR Lead Time:           avg %.1fh  p50 %.1fh  p90 %.1fh  (%d PRs)\n", avgH, p50H, p90H, prCount)

	rwCount, rwAvgH, err := m.ReviewWaitTime(ctx, repoID, from, to)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Review Wait Time:       avg %.1fh (%d PRs)\n", rwAvgH, rwCount)

	dist, err := m.PRSizeDistribution(ctx, repoID, from, to)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "PR Size Distribution:   %s\n", formatSizeDist(dist))

	days, err := m.DailyBuildDuration(ctx, repoID, from, to)
	if err != nil {
		return err
	}
	if len(days) > 0 {
		fmt.Fprintf(w, "\nDaily Build Duration (avg seconds):\n")
		for _, d := range days {
			fmt.Fprintf(w, "  %s: %.0fs (%d builds)\n", d.Day, d.AvgSeconds, d.Count)
		}
	}

	return printDORA(ctx, w, m, r, from, to)
}

// printDORA renders the DORA section. Deployments are merges into the
// default branch, so without a known default branch there is nothing
// honest to report — the section says so instead of guessing.
func printDORA(ctx context.Context, w io.Writer, m *persistence.MetricsPersister, r repo.Repo, from, to time.Time) error {
	fmt.Fprintf(w, "\nDORA (deployment = PR merged into default branch)\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 40))

	if r.DefaultBranch == "" {
		fmt.Fprintf(w, "Default branch unknown; run `devpulse repo refresh %s` first.\n", r.Name.String())
		return nil
	}

	in, err := m.DORAInput(ctx, r.ID, r.DefaultBranch, from, to)
	if err != nil {
		return err
	}
	in.HotfixLabel = r.HotfixLabel
	in.To = elapsedEnd(to, time.Now().UTC())
	rep := dora.Compute(in)

	fmt.Fprintf(w, "Deployment Frequency:   %d deploys into %s  (%.2f/week, %d deploy days)\n",
		rep.Deployments, r.DefaultBranch, rep.PerWeek, rep.DeployDays)
	fmt.Fprintf(w, "Lead Time for Changes:  %s\n", formatSummary(rep.LeadTime, "deploys"))

	cfr := "n/a (no deploys)"
	if rep.ChangeFailureRate != nil {
		cfr = fmt.Sprintf("%.1f%% (%d/%d)", *rep.ChangeFailureRate*100, rep.Reverts+rep.Hotfixes, rep.Deployments)
	}
	fmt.Fprintf(w, "Change Failure Rate:    %s  reverts=%d hotfixes=%d (label %q)\n",
		cfr, rep.Reverts, rep.Hotfixes, r.HotfixLabel)
	fmt.Fprintf(w, "Recovery Time:          %s  from reverts=%d incidents=%d (label %q)\n",
		formatSummary(rep.Recovery, "samples"), rep.RecoveryFromReverts, rep.RecoveryFromIncidents, r.IncidentLabel)
	return nil
}

// elapsedEnd clamps a window end to now. A window still in progress —
// the default, since --from defaults to the current month — is measured
// only up to now, so the per-week rate is not diluted by days that have
// not happened yet. Nothing can be merged after now, so the counts are
// unaffected.
func elapsedEnd(to, now time.Time) time.Time {
	if now.Before(to) {
		return now
	}
	return to
}

// formatSummary renders an hours Summary, or "(no data)" for an empty
// sample so a zero average is never mistaken for an instant result.
func formatSummary(s statx.Summary, unit string) string {
	if s.Count == 0 {
		return "(no data)"
	}
	return fmt.Sprintf("avg %.1fh  p50 %.1fh  p90 %.1fh  (%d %s)", s.Avg, s.P50, s.P90, s.Count, unit)
}

func formatSizeDist(dist map[string]int) string {
	var parts []string
	for _, b := range pullrequest.SizeBuckets() {
		if n, ok := dist[b]; ok {
			parts = append(parts, fmt.Sprintf("%s:%d", b, n))
		}
	}
	if n, ok := dist["unknown"]; ok {
		parts = append(parts, fmt.Sprintf("unknown:%d", n))
	}
	if len(parts) == 0 {
		return "(no data)"
	}
	return strings.Join(parts, "  ")
}

func parseMonth(s string) (time.Time, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM, got %q", s)
	}
	return t, nil
}
