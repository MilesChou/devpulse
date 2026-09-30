package metrics

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/mileschou/devpulse/internal/people"
	"github.com/mileschou/devpulse/internal/repo"
)

// SparklineMonths is how many monthly summaries each overview row
// carries, ending at the period's end.
const SparklineMonths = 12

// overviewWorkers bounds how many rows are computed at once. Rows are
// independent read-only report runs; a few in parallel keep an overview
// fast without flooding the database.
const overviewWorkers = 4

// Summary is one comparison cell set: the columns the Overview tables
// show. A metric is null when the period has no data for it, so a
// client never shows a misleading 0. Counts are plain numbers: zero PRs
// is data.
type Summary struct {
	PRsOpened int `json:"prs_opened"`
	PRsMerged int `json:"prs_merged"`
	// LeadTimeP50Hours is the median, not the mean: a few PRs left open
	// for weeks would otherwise dominate the comparison.
	LeadTimeP50Hours *float64 `json:"lead_time_p50_hours"`
	BuildsPerPR      *float64 `json:"builds_per_pr"`
	CIFailureRate    *float64 `json:"ci_failure_rate"` // 0..1
	// BuildP50Seconds is the median build duration, like the lead time.
	BuildP50Seconds *float64 `json:"build_p50_seconds"`
	ReviewWaitHours *float64 `json:"review_wait_hours"`
	// DeploysPerWeek is set for repo rows only: DORA is per repo.
	DeploysPerWeek *float64 `json:"deploys_per_week"`
}

// SummaryOf condenses a report into a Summary.
func SummaryOf(r Report) Summary {
	s := Summary{PRsMerged: r.PRLeadTime.Count}
	for _, b := range r.PRSizeDistribution {
		s.PRsOpened += b.Count
	}
	if r.PRLeadTime.Count > 0 {
		s.LeadTimeP50Hours = ptr(r.PRLeadTime.P50Hours)
	}
	if r.AvgBuildsPerPR > 0 {
		s.BuildsPerPR = ptr(r.AvgBuildsPerPR)
	}
	if r.BuildFailure.Total > 0 {
		s.CIFailureRate = ptr(r.BuildFailure.Rate)
	}
	if r.BuildDuration.Count > 0 {
		s.BuildP50Seconds = ptr(r.BuildDuration.P50Seconds)
	}
	if r.ReviewWait.Count > 0 {
		s.ReviewWaitHours = ptr(r.ReviewWait.AvgHours)
	}
	if r.DORA != nil {
		s.DeploysPerWeek = ptr(r.DORA.PerWeek)
	}
	return s
}

func ptr(v float64) *float64 { return &v }

// Previous is the window of the same number of months right before w.
func (w Window) Previous() Window {
	return Window{From: w.From.AddDate(0, -w.Months(), 0), To: w.From}
}

// sparkline is the SparklineMonths window ending where w ends.
func (w Window) sparkline() Window {
	return Window{From: w.To.AddDate(0, -SparklineMonths, 0), To: w.To}
}

// Period is a [From, To) month range in YYYY-MM form.
type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (w Window) period() Period {
	return Period{From: w.From.Format(monthLayout), To: w.To.Format(monthLayout)}
}

// MonthSummary is one point of a row's sparkline.
type MonthSummary struct {
	Month   string  `json:"month"` // YYYY-MM
	Summary Summary `json:"summary"`
}

// Comparison is what every overview row carries.
type Comparison struct {
	Current  Summary        `json:"current"`
	Previous Summary        `json:"previous"`
	Monthly  []MonthSummary `json:"monthly"`
}

// RepoRow is one repo of the repo overview.
type RepoRow struct {
	Repo string `json:"repo"`
	Comparison
}

// MemberRow is one member, or one active account no member claims
// (MemberID nil, Name = the account), of the member overview.
type MemberRow struct {
	MemberID *string  `json:"member_id"`
	Name     string   `json:"name"`
	Accounts []string `json:"accounts"`
	Comparison
}

// Overview is the answer of an overview endpoint.
type Overview[R any] struct {
	Period
	Previous Period `json:"previous"`
	Rows     []R    `json:"rows"`
}

func newOverview[R any](w Window, rows []R) Overview[R] {
	return Overview[R]{Period: w.period(), Previous: w.Previous().period(), Rows: rows}
}

// compare computes the current, previous and monthly summaries of one
// row.
func compare(ctx context.Context, src Source, t Target, w Window, now time.Time, scope *Scope) (Comparison, error) {
	cur, err := Compute(ctx, src, t, w, now, scope)
	if err != nil {
		return Comparison{}, err
	}
	prev, err := Compute(ctx, src, t, w.Previous(), now, scope)
	if err != nil {
		return Comparison{}, err
	}
	months, err := ComputeMonthly(ctx, src, t, w.sparkline(), now, scope)
	if err != nil {
		return Comparison{}, err
	}
	c := Comparison{
		Current:  SummaryOf(cur),
		Previous: SummaryOf(prev),
		Monthly:  make([]MonthSummary, len(months)),
	}
	for i, m := range months {
		c.Monthly[i] = MonthSummary{Month: m.From, Summary: SummaryOf(m)}
	}
	return c, nil
}

// ComputeRepoOverview compares repos: one row per repo, in the order
// given, each with its own DORA-based deployments per week.
func ComputeRepoOverview(ctx context.Context, src Source, repos []repo.Repo, w Window, now time.Time) (Overview[RepoRow], error) {
	if err := w.CheckTrend(); err != nil {
		return Overview[RepoRow]{}, err
	}
	rows := make([]RepoRow, len(repos))
	err := forEach(ctx, len(repos), func(ctx context.Context, i int) error {
		c, err := compare(ctx, src, Single(repos[i]), w, now, nil)
		rows[i] = RepoRow{Repo: repos[i].Name.String(), Comparison: c}
		return err
	})
	if err != nil {
		return Overview[RepoRow]{}, err
	}
	return newOverview(w, rows), nil
}

// ComputeMemberOverview compares people across all repos: members with
// activity in the current or previous period (by name), then active
// accounts no member claims. Excluded accounts are never active (see
// AuthorLister).
func ComputeMemberOverview(
	ctx context.Context,
	authors AuthorLister,
	scoped ScopedSource,
	members []people.Member,
	repos []repo.Repo,
	w Window,
	now time.Time,
) (Overview[MemberRow], error) {
	if err := w.CheckTrend(); err != nil {
		return Overview[MemberRow]{}, err
	}
	t := AllRepos(repos)
	active, err := authors.Authors(ctx, t.ids(), w.Previous().From, w.To)
	if err != nil {
		return Overview[MemberRow]{}, err
	}

	type pending struct {
		row   MemberRow
		scope *Scope
	}
	var todo []pending
	claimed := map[string]bool{}
	for _, m := range members {
		for _, a := range m.Accounts {
			claimed[a] = true
		}
		if !slices.ContainsFunc(m.Accounts, func(a string) bool { return slices.Contains(active, a) }) {
			continue
		}
		id := m.ID
		todo = append(todo, pending{
			row:   MemberRow{MemberID: &id, Name: m.DisplayName, Accounts: m.Accounts},
			scope: &Scope{Kind: "member", ID: m.ID, Name: m.DisplayName, Accounts: m.Accounts},
		})
	}
	for _, a := range active {
		if claimed[a] {
			continue
		}
		accounts := []string{a}
		todo = append(todo, pending{
			row:   MemberRow{Name: a, Accounts: accounts},
			scope: &Scope{Kind: "account", ID: a, Name: a, Accounts: accounts},
		})
	}

	rows := make([]MemberRow, len(todo))
	err = forEach(ctx, len(todo), func(ctx context.Context, i int) error {
		p := todo[i]
		c, err := compare(ctx, scoped(p.scope.Accounts), t, w, now, p.scope)
		p.row.Comparison = c
		rows[i] = p.row
		return err
	})
	if err != nil {
		return Overview[MemberRow]{}, err
	}
	return newOverview(w, rows), nil
}

// forEach runs fn for 0..n-1 on at most overviewWorkers goroutines and
// returns the first error; the others are cancelled through ctx.
func forEach(ctx context.Context, n int, fn func(context.Context, int) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	jobs := make(chan int)
	for range min(overviewWorkers, n) {
		wg.Go(func() {
			for i := range jobs {
				if err := fn(ctx, i); err != nil {
					once.Do(func() { firstErr = err; cancel() })
				}
			}
		})
	}
	for i := range n {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return firstErr
}
