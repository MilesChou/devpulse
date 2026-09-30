// Package metrics assembles the engineering-efficiency report for one
// repo and month window. It is the single place that turns persister
// queries into a Report, so the `devpulse metrics` CLI output and the
// HTTP API JSON can never disagree on what a metric means.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mileschou/devpulse/internal/dora"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/statx"
)

// monthLayout is the YYYY-MM form used by the CLI flags, the API query
// parameters, and the Report window labels.
const monthLayout = "2006-01"

// MaxMonths bounds the width of a monthly trend (ComputeMonthly) so one
// API call cannot fan out into an unbounded number of per-month
// queries. Ten years covers a long-lived repo's whole history in one
// chart. A single-window Compute runs one fixed set of queries at any
// width, so it has no limit.
const MaxMonths = 120

// unknownBucket labels PRs whose size_bucket column is NULL.
const unknownBucket = "unknown"

// Source is the subset of persistence.MetricsPersister the report
// needs. Declared here, with the consumer, so tests can substitute it.
type Source interface {
	BuildFailureRate(ctx context.Context, repoIDs []string, from, to time.Time) (total, failed int, rate float64, err error)
	AverageBuildsPerPR(ctx context.Context, repoIDs []string, from, to time.Time) (float64, error)
	PRLeadTime(ctx context.Context, repoIDs []string, from, to time.Time) (count int, avgHours, p50Hours, p90Hours float64, err error)
	ReviewWaitTime(ctx context.Context, repoIDs []string, from, to time.Time) (count int, avgHours float64, err error)
	PRSizeDistribution(ctx context.Context, repoIDs []string, from, to time.Time) (map[string]int, error)
	BuildDurations(ctx context.Context, repoIDs []string, from, to time.Time) ([]persistence.DayDuration, statx.Summary, error)
	DORAInput(ctx context.Context, repoID, defaultBranch string, from, to time.Time) (dora.Input, error)
}

var _ Source = (*persistence.MetricsPersister)(nil)

// Report is every metric for one repo, or all repos, over the [From, To)
// month window.
// The JSON shape is the HTTP API contract; the desktop dashboard
// decodes it, so renaming a tag is a breaking change for that client.
type Report struct {
	Repo               string             `json:"repo"`
	From               string             `json:"from"` // YYYY-MM, inclusive
	To                 string             `json:"to"`   // YYYY-MM, exclusive
	BuildFailure       BuildFailure       `json:"build_failure"`
	AvgBuildsPerPR     float64            `json:"avg_builds_per_pr"`
	PRLeadTime         HoursSummary       `json:"pr_lead_time"`
	ReviewWait         ReviewWait         `json:"review_wait"`
	PRSizeDistribution []SizeBucketCount  `json:"pr_size_distribution"`
	DailyBuildDuration []DayBuildDuration `json:"daily_build_duration"`
	// BuildDuration summarizes every build of the window. The median is
	// the headline: a few very slow builds would dominate the mean.
	BuildDuration BuildDuration `json:"build_duration"`

	// DORA is nil (JSON null) when the repo's default branch is not
	// known yet: deployments are merges into it, so there is nothing
	// honest to report until `devpulse repo refresh` fills it in. It is
	// also nil for a scoped report: DORA measures the delivery of the
	// whole repo, so it is not split by person.
	DORA *DORA `json:"dora"`

	// Scope is the member or team the report is limited to; nil means
	// everyone.
	Scope *Scope `json:"scope"`
}

// Scope names whose work a report covers. The Source passed alongside
// must already be limited to Accounts.
type Scope struct {
	Kind     string   `json:"kind"` // "member", "team", or "account" (an unmapped account)
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Accounts []string `json:"accounts"`
}

// BuildFailure is the CI failure rate over PR-triggered builds.
type BuildFailure struct {
	Total  int     `json:"total"`
	Failed int     `json:"failed"`
	Rate   float64 `json:"rate"` // 0..1
}

// HoursSummary is the avg / p50 / p90 of a sample of durations, in
// hours. All zero with Count 0 means "no data", not "instant".
type HoursSummary struct {
	Count    int     `json:"count"`
	AvgHours float64 `json:"avg_hours"`
	P50Hours float64 `json:"p50_hours"`
	P90Hours float64 `json:"p90_hours"`
}

func fromSummary(s statx.Summary) HoursSummary {
	return HoursSummary{Count: s.Count, AvgHours: s.Avg, P50Hours: s.P50, P90Hours: s.P90}
}

// DORA is the four DORA metrics for the window; see internal/dora for
// the definitions. A deployment is a PR merged into DefaultBranch.
type DORA struct {
	DefaultBranch string `json:"default_branch"`
	HotfixLabel   string `json:"hotfix_label"`
	IncidentLabel string `json:"incident_label"`

	Deployments int     `json:"deployments"`
	PerWeek     float64 `json:"per_week"`
	DeployDays  int     `json:"deploy_days"`

	LeadTime HoursSummary `json:"lead_time"`

	Reverts  int `json:"reverts"`
	Hotfixes int `json:"hotfixes"`
	// ChangeFailureRate is null when there were no deployments: 0/0 is
	// "not applicable", not a perfect score.
	ChangeFailureRate *float64 `json:"change_failure_rate"`

	Recovery              HoursSummary `json:"recovery"`
	RecoveryFromReverts   int          `json:"recovery_from_reverts"`
	RecoveryFromIncidents int          `json:"recovery_from_incidents"`
}

// ReviewWait is the ready → first review duration, in hours.
type ReviewWait struct {
	Count    int     `json:"count"`
	AvgHours float64 `json:"avg_hours"`
}

// SizeBucketCount is one bar of the PR size distribution.
type SizeBucketCount struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

// DayBuildDuration is the build duration of one UTC day.
type DayBuildDuration struct {
	Day        string  `json:"day"` // YYYY-MM-DD
	AvgSeconds float64 `json:"avg_seconds"`
	P50Seconds float64 `json:"p50_seconds"`
	Count      int     `json:"count"`
}

// BuildDuration is the count, mean, median and p90 of the builds of a
// window, in seconds. All zero with Count 0 means no builds.
type BuildDuration struct {
	Count      int     `json:"count"`
	AvgSeconds float64 `json:"avg_seconds"`
	P50Seconds float64 `json:"p50_seconds"`
	P90Seconds float64 `json:"p90_seconds"`
}

// Window is a [From, To) range of whole months, both at 00:00 UTC on
// the first of the month.
type Window struct {
	From time.Time
	To   time.Time
}

// ErrInvalidWindow classifies caller mistakes (bad month format, empty
// or oversized range) so the HTTP layer can answer 400 instead of 500.
var ErrInvalidWindow = errors.New("metrics: invalid window")

// ParseMonth parses YYYY-MM into 00:00 UTC on the first of that month.
func ParseMonth(s string) (time.Time, error) {
	t, err := time.Parse(monthLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: expected YYYY-MM, got %q", ErrInvalidWindow, s)
	}
	return t, nil
}

// ParseWindow resolves the from/to month strings with the CLI's
// defaults: an empty from is the month of now, and an empty to is one
// month after from. The result spans at least one month.
func ParseWindow(from, to string, now time.Time) (Window, error) {
	var w Window
	var err error

	if from == "" {
		now = now.UTC()
		w.From = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else if w.From, err = ParseMonth(from); err != nil {
		return Window{}, err
	}

	if to == "" {
		w.To = w.From.AddDate(0, 1, 0)
	} else if w.To, err = ParseMonth(to); err != nil {
		return Window{}, err
	}

	if !w.To.After(w.From) {
		return Window{}, fmt.Errorf("%w: to (%s) must be after from (%s)",
			ErrInvalidWindow, w.To.Format(monthLayout), w.From.Format(monthLayout))
	}
	return w, nil
}

// CheckTrend reports whether w is narrow enough for ComputeMonthly.
func (w Window) CheckTrend() error {
	if n := w.Months(); n > MaxMonths {
		return fmt.Errorf("%w: %d months exceeds the %d-month limit for a monthly trend", ErrInvalidWindow, n, MaxMonths)
	}
	return nil
}

// Months returns the number of whole months in the window.
func (w Window) Months() int {
	return (w.To.Year()-w.From.Year())*12 + int(w.To.Month()) - int(w.From.Month())
}

// Label renders the window the way the CLI header does: "2026-01" for a
// single month, "2026-01 ~ 2026-03" for a wider range, so a multi-month
// aggregate is not mistaken for one month's numbers.
func (w Window) Label() string {
	label := w.From.Format(monthLayout)
	if lastMonth := w.To.AddDate(0, -1, 0); lastMonth.After(w.From) {
		label = fmt.Sprintf("%s ~ %s", w.From.Format(monthLayout), lastMonth.Format(monthLayout))
	}
	return label
}

// ElapsedEnd clamps a window end to now. A window still in progress
// (the default, since the window defaults to the current month) is
// measured only up to now, so a per-week rate is not diluted by days
// that have not happened yet. Nothing can be merged after now, so the
// counts are unaffected.
func ElapsedEnd(to, now time.Time) time.Time {
	if now.Before(to) {
		return now
	}
	return to
}

// Compute runs every metric query for the target over the window. now
// bounds a window that is still in progress (see ElapsedEnd). scope is
// nil for everyone; otherwise src must be limited to scope.Accounts.
func Compute(ctx context.Context, src Source, t Target, w Window, now time.Time, scope *Scope) (Report, error) {
	repoIDs := t.ids()
	r := Report{
		Repo:  t.Name(),
		Scope: scope,
		From:  w.From.Format(monthLayout),
		To:    w.To.Format(monthLayout),
	}

	total, failed, rate, err := src.BuildFailureRate(ctx, repoIDs, w.From, w.To)
	if err != nil {
		return Report{}, err
	}
	r.BuildFailure = BuildFailure{Total: total, Failed: failed, Rate: rate}

	if r.AvgBuildsPerPR, err = src.AverageBuildsPerPR(ctx, repoIDs, w.From, w.To); err != nil {
		return Report{}, err
	}

	count, avgH, p50H, p90H, err := src.PRLeadTime(ctx, repoIDs, w.From, w.To)
	if err != nil {
		return Report{}, err
	}
	r.PRLeadTime = HoursSummary{Count: count, AvgHours: avgH, P50Hours: p50H, P90Hours: p90H}

	rwCount, rwAvgH, err := src.ReviewWaitTime(ctx, repoIDs, w.From, w.To)
	if err != nil {
		return Report{}, err
	}
	r.ReviewWait = ReviewWait{Count: rwCount, AvgHours: rwAvgH}

	dist, err := src.PRSizeDistribution(ctx, repoIDs, w.From, w.To)
	if err != nil {
		return Report{}, err
	}
	r.PRSizeDistribution = orderSizeDistribution(dist)

	days, builds, err := src.BuildDurations(ctx, repoIDs, w.From, w.To)
	if err != nil {
		return Report{}, err
	}
	r.DailyBuildDuration = make([]DayBuildDuration, 0, len(days))
	for _, d := range days {
		r.DailyBuildDuration = append(r.DailyBuildDuration, DayBuildDuration{
			Day:        d.Day,
			AvgSeconds: d.AvgSeconds,
			P50Seconds: d.P50Seconds,
			Count:      d.Count,
		})
	}
	r.BuildDuration = BuildDuration{
		Count:      builds.Count,
		AvgSeconds: builds.Avg,
		P50Seconds: builds.P50,
		P90Seconds: builds.P90,
	}

	if rp, ok := t.single(); ok && rp.DefaultBranch != "" && scope == nil {
		if r.DORA, err = computeDORA(ctx, src, rp, w, now); err != nil {
			return Report{}, err
		}
	}

	return r, nil
}

func computeDORA(ctx context.Context, src Source, rp repo.Repo, w Window, now time.Time) (*DORA, error) {
	in, err := src.DORAInput(ctx, rp.ID, rp.DefaultBranch, w.From, w.To)
	if err != nil {
		return nil, err
	}
	in.HotfixLabel = rp.HotfixLabel
	in.To = ElapsedEnd(w.To, now.UTC())
	rep := dora.Compute(in)

	return &DORA{
		DefaultBranch:         rp.DefaultBranch,
		HotfixLabel:           rp.HotfixLabel,
		IncidentLabel:         rp.IncidentLabel,
		Deployments:           rep.Deployments,
		PerWeek:               rep.PerWeek,
		DeployDays:            rep.DeployDays,
		LeadTime:              fromSummary(rep.LeadTime),
		Reverts:               rep.Reverts,
		Hotfixes:              rep.Hotfixes,
		ChangeFailureRate:     rep.ChangeFailureRate,
		Recovery:              fromSummary(rep.Recovery),
		RecoveryFromReverts:   rep.RecoveryFromReverts,
		RecoveryFromIncidents: rep.RecoveryFromIncidents,
	}, nil
}

// ComputeMonthly returns one Report per month of the window, oldest
// first, for month-over-month trends.
func ComputeMonthly(ctx context.Context, src Source, t Target, w Window, now time.Time, scope *Scope) ([]Report, error) {
	if err := w.CheckTrend(); err != nil {
		return nil, err
	}
	out := make([]Report, 0, w.Months())
	for m := w.From; m.Before(w.To); m = m.AddDate(0, 1, 0) {
		r, err := Compute(ctx, src, t, Window{From: m, To: m.AddDate(0, 1, 0)}, now, scope)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// orderSizeDistribution turns the bucket → count map into a slice in
// ascending size order. Every standard bucket is present (zero when it
// has no PRs) so charts keep a stable x-axis; "unknown" is appended
// only when some PRs lack a bucket.
func orderSizeDistribution(dist map[string]int) []SizeBucketCount {
	buckets := pullrequest.SizeBuckets()
	out := make([]SizeBucketCount, 0, len(buckets)+1)
	for _, b := range buckets {
		out = append(out, SizeBucketCount{Bucket: b, Count: dist[b]})
	}
	if n := dist[unknownBucket]; n > 0 {
		out = append(out, SizeBucketCount{Bucket: unknownBucket, Count: n})
	}
	return out
}
