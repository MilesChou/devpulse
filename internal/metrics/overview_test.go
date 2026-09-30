package metrics

import (
	"context"
	"errors"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/dora"
	"github.com/mileschou/devpulse/internal/people"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
)

func TestSummaryOf(t *testing.T) {
	r := Report{
		BuildFailure:       BuildFailure{Total: 4, Failed: 1, Rate: 0.25},
		AvgBuildsPerPR:     1.5,
		PRLeadTime:         HoursSummary{Count: 2, AvgHours: 20},
		ReviewWait:         ReviewWait{Count: 1, AvgHours: 3},
		PRSizeDistribution: []SizeBucketCount{{Bucket: "XS", Count: 2}, {Bucket: "L", Count: 1}},
		// 3 builds of 60 s on one day, 1 of 300 s on another: 120 s per
		// build, not the 180 s average of the two days.
		DailyBuildDuration: []DayBuildDuration{
			{Day: "2026-05-01", AvgSeconds: 60, Count: 3},
			{Day: "2026-05-02", AvgSeconds: 300, Count: 1},
		},
		DORA: &DORA{PerWeek: 2.5},
	}
	s := SummaryOf(r)
	if s.PRsOpened != 3 || s.PRsMerged != 2 {
		t.Fatalf("counts: %+v", s)
	}
	for name, got := range map[string]*float64{
		"lead": s.LeadTimeP50Hours, "builds": s.BuildsPerPR, "failure": s.CIFailureRate,
		"build seconds": s.AvgBuildSeconds, "review": s.ReviewWaitHours, "deploys": s.DeploysPerWeek,
	} {
		if got == nil {
			t.Fatalf("%s is nil", name)
		}
	}
	if math.Abs(*s.AvgBuildSeconds-120) > 1e-9 {
		t.Fatalf("avg build seconds: got %v, want 120", *s.AvgBuildSeconds)
	}

	empty := SummaryOf(Report{})
	if empty.LeadTimeP50Hours != nil || empty.BuildsPerPR != nil || empty.CIFailureRate != nil ||
		empty.AvgBuildSeconds != nil || empty.ReviewWaitHours != nil || empty.DeploysPerWeek != nil {
		t.Fatalf("no data must be null, got %+v", empty)
	}
}

func TestWindow_Previous(t *testing.T) {
	for _, tt := range []struct{ from, to, prevFrom, prevTo string }{
		{"2026-09", "2026-10", "2026-08", "2026-09"},
		{"2026-07", "2026-10", "2026-04", "2026-07"},
		{"2026-01", "2026-02", "2025-12", "2026-01"},
	} {
		w, err := ParseWindow(tt.from, tt.to, now)
		if err != nil {
			t.Fatal(err)
		}
		p := w.Previous().period()
		if p.From != tt.prevFrom || p.To != tt.prevTo {
			t.Fatalf("%s..%s: previous %v, want %s..%s", tt.from, tt.to, p, tt.prevFrom, tt.prevTo)
		}
	}
}

// overviewSource is safe for the overview's parallel rows. Its failure
// total is the window's month number, and it counts the reports run.
type overviewSource struct {
	mu      sync.Mutex
	reports int
	fail    error
}

func (s *overviewSource) BuildFailureRate(_ context.Context, _ []string, from, _ time.Time) (int, int, float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports++
	return int(from.Month()), 0, 0, s.fail
}
func (*overviewSource) AverageBuildsPerPR(context.Context, []string, time.Time, time.Time) (float64, error) {
	return 0, nil
}
func (*overviewSource) PRLeadTime(context.Context, []string, time.Time, time.Time) (int, float64, float64, float64, error) {
	return 0, 0, 0, 0, nil
}
func (*overviewSource) ReviewWaitTime(context.Context, []string, time.Time, time.Time) (int, float64, error) {
	return 0, 0, nil
}
func (*overviewSource) PRSizeDistribution(context.Context, []string, time.Time, time.Time) (map[string]int, error) {
	return nil, nil
}
func (*overviewSource) DailyBuildDuration(context.Context, []string, time.Time, time.Time) ([]persistence.DayDuration, error) {
	return nil, nil
}
func (*overviewSource) DORAInput(_ context.Context, _, _ string, from, to time.Time) (dora.Input, error) {
	return dora.Input{From: from, To: to}, nil
}

func testRepo(t *testing.T, name string) repo.Repo {
	t.Helper()
	fn, err := repo.ParseFullName(name)
	if err != nil {
		t.Fatal(err)
	}
	return repo.Repo{ID: "id-" + name, Name: fn, DefaultBranch: "main"}
}

func TestComputeRepoOverview(t *testing.T) {
	src := &overviewSource{}
	repos := []repo.Repo{testRepo(t, "acme/a"), testRepo(t, "acme/b"), testRepo(t, "acme/c")}
	w, _ := ParseWindow("2026-07", "2026-10", now)

	ov, err := ComputeRepoOverview(context.Background(), src, repos, w, now)
	if err != nil {
		t.Fatal(err)
	}
	if ov.From != "2026-07" || ov.To != "2026-10" || ov.Previous != (Period{"2026-04", "2026-07"}) {
		t.Fatalf("periods: %+v / %+v", ov.Period, ov.Previous)
	}
	if len(ov.Rows) != 3 || ov.Rows[1].Repo != "acme/b" {
		t.Fatalf("rows in repo order: %+v", ov.Rows)
	}
	row := ov.Rows[0]
	if len(row.Monthly) != SparklineMonths || row.Monthly[0].Month != "2025-10" || row.Monthly[11].Month != "2026-09" {
		t.Fatalf("sparkline months: first %v last %v (%d)", row.Monthly[0].Month, row.Monthly[len(row.Monthly)-1].Month, len(row.Monthly))
	}
	// Repo rows are single-repo reports, so DORA gives deploys per week.
	if row.Current.DeploysPerWeek == nil {
		t.Fatal("repo row must carry deploys per week")
	}
	if want := 3 * (2 + SparklineMonths); src.reports != want {
		t.Fatalf("reports run: got %d, want %d", src.reports, want)
	}

	src.fail = errors.New("boom")
	if _, err := ComputeRepoOverview(context.Background(), src, repos, w, now); err == nil {
		t.Fatal("a failing row must fail the overview")
	}
}

type fakeAuthors struct{ active []string }

func (f fakeAuthors) Authors(context.Context, []string, time.Time, time.Time) ([]string, error) {
	return f.active, nil
}

func TestComputeMemberOverview(t *testing.T) {
	src := &overviewSource{}
	var scopedWith [][]string
	var mu sync.Mutex
	scoped := func(accounts []string) Source {
		mu.Lock()
		scopedWith = append(scopedWith, accounts)
		mu.Unlock()
		return src
	}
	members := []people.Member{
		{ID: "m1", DisplayName: "Alice", Accounts: []string{"alice", "alice-work"}},
		{ID: "m2", DisplayName: "Idle", Accounts: []string{"idle"}},
	}
	repos := []repo.Repo{testRepo(t, "acme/a"), testRepo(t, "acme/b")}
	w, _ := ParseWindow("2026-09", "", now)

	ov, err := ComputeMemberOverview(context.Background(),
		fakeAuthors{active: []string{"alice-work", "zed"}}, scoped, members, repos, w, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Rows) != 2 {
		t.Fatalf("want Alice and unmapped zed, got %+v", ov.Rows)
	}
	alice, zed := ov.Rows[0], ov.Rows[1]
	if alice.MemberID == nil || *alice.MemberID != "m1" || alice.Name != "Alice" {
		t.Fatalf("first row: %+v", alice)
	}
	if zed.MemberID != nil || zed.Name != "zed" || !slices.Equal(zed.Accounts, []string{"zed"}) {
		t.Fatalf("unmapped row: %+v", zed)
	}
	// Member rows pool all repos, so there is no DORA.
	if alice.Current.DeploysPerWeek != nil {
		t.Fatal("member rows have no deploys per week")
	}
	for _, acc := range scopedWith {
		if slices.Contains(acc, "idle") {
			t.Fatal("an inactive member must not be computed")
		}
	}
}
