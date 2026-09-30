package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/dora"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
)

var now = time.Date(2026, 5, 17, 13, 0, 0, 0, time.UTC)

func TestParseWindow(t *testing.T) {
	month := func(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }

	tests := []struct {
		name     string
		from, to string
		want     Window
		wantErr  bool
	}{
		{"defaults to current month", "", "", Window{month(2026, 5), month(2026, 6)}, false},
		{"from only", "2026-01", "", Window{month(2026, 1), month(2026, 2)}, false},
		{"explicit range", "2025-11", "2026-02", Window{month(2025, 11), month(2026, 2)}, false},
		{"bad from", "2026/01", "", Window{}, true},
		{"bad to", "2026-01", "soon", Window{}, true},
		{"empty range", "2026-03", "2026-03", Window{}, true},
		{"reversed range", "2026-03", "2026-01", Window{}, true},
		// Single windows have no width limit; only monthly trends do.
		{"wide", "2020-01", "2026-01", Window{month(2020, 1), month(2026, 1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseWindow(tt.from, tt.to, now)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidWindow) {
					t.Fatalf("want ErrInvalidWindow, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.From.Equal(tt.want.From) || !got.To.Equal(tt.want.To) {
				t.Fatalf("got %v..%v, want %v..%v", got.From, got.To, tt.want.From, tt.want.To)
			}
		})
	}
}

func TestWindow_CheckTrend(t *testing.T) {
	ok, _ := ParseWindow("2023-01", "2026-01", now)
	if err := ok.CheckTrend(); err != nil {
		t.Fatalf("36 months must pass: %v", err)
	}
	wide, _ := ParseWindow("2023-01", "2026-02", now)
	if err := wide.CheckTrend(); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("37 months: want ErrInvalidWindow, got %v", err)
	}
	if _, err := ComputeMonthly(context.Background(), &fakeSource{}, repo.Repo{}, wide, now); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("ComputeMonthly must enforce the limit, got %v", err)
	}
}

func TestWindow_Label(t *testing.T) {
	one, _ := ParseWindow("2026-01", "", now)
	if got := one.Label(); got != "2026-01" {
		t.Fatalf("single month label: %q", got)
	}
	three, _ := ParseWindow("2026-01", "2026-04", now)
	if got := three.Label(); got != "2026-01 ~ 2026-03" {
		t.Fatalf("range label: %q", got)
	}
	if three.Months() != 3 {
		t.Fatalf("months: %d", three.Months())
	}
}

func TestOrderSizeDistribution(t *testing.T) {
	got := orderSizeDistribution(map[string]int{"L": 2, "XS": 1, "unknown": 3})
	want := []SizeBucketCount{
		{"XS", 1}, {"S", 0}, {"M", 0}, {"L", 2}, {"XL", 0}, {"unknown", 3},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]: got %v, want %v", i, got[i], want[i])
		}
	}

	// No unknown bar when every PR has a bucket.
	if got := orderSizeDistribution(nil); len(got) != 5 {
		t.Fatalf("empty dist: got %v", got)
	}
}

// fakeSource records the windows it was asked for and returns the
// window's month number as the failure total, so a test can tell the
// per-month reports apart.
type fakeSource struct {
	windows      []Window
	doraBranches []string
}

func (f *fakeSource) BuildFailureRate(_ context.Context, _ string, from, to time.Time) (int, int, float64, error) {
	f.windows = append(f.windows, Window{from, to})
	return int(from.Month()), 0, 0, nil
}
func (f *fakeSource) AverageBuildsPerPR(context.Context, string, time.Time, time.Time) (float64, error) {
	return 0, nil
}
func (f *fakeSource) PRLeadTime(context.Context, string, time.Time, time.Time) (int, float64, float64, float64, error) {
	return 0, 0, 0, 0, nil
}
func (f *fakeSource) ReviewWaitTime(context.Context, string, time.Time, time.Time) (int, float64, error) {
	return 0, 0, nil
}
func (f *fakeSource) PRSizeDistribution(context.Context, string, time.Time, time.Time) (map[string]int, error) {
	return nil, nil
}
func (f *fakeSource) DailyBuildDuration(context.Context, string, time.Time, time.Time) ([]persistence.DayDuration, error) {
	return nil, nil
}
func (f *fakeSource) DORAInput(_ context.Context, _, branch string, from, to time.Time) (dora.Input, error) {
	f.doraBranches = append(f.doraBranches, branch)
	return dora.Input{From: from, To: to}, nil
}

func TestComputeMonthly(t *testing.T) {
	src := &fakeSource{}
	w, err := ParseWindow("2025-11", "2026-02", now)
	if err != nil {
		t.Fatalf("window: %v", err)
	}

	rp := repo.Repo{ID: "id", Name: repo.FullName{Owner: "o", Name: "r"}}
	got, err := ComputeMonthly(context.Background(), src, rp, w, now)
	if err != nil {
		t.Fatalf("ComputeMonthly: %v", err)
	}

	wantMonths := []string{"2025-11", "2025-12", "2026-01"}
	if len(got) != len(wantMonths) {
		t.Fatalf("got %d reports, want %d", len(got), len(wantMonths))
	}
	for i, m := range wantMonths {
		if got[i].From != m {
			t.Fatalf("[%d] from: got %s, want %s", i, got[i].From, m)
		}
		if got[i].BuildFailure.Total != int(src.windows[i].From.Month()) {
			t.Fatalf("[%d] report not computed for its own month", i)
		}
		if got[i].DailyBuildDuration == nil {
			t.Fatalf("[%d] daily durations must encode as [] not null", i)
		}
		if got[i].DORA != nil {
			t.Fatalf("[%d] DORA must be nil without a default branch", i)
		}
	}
	if len(src.doraBranches) != 0 {
		t.Fatalf("DORA queried without a default branch: %v", src.doraBranches)
	}
}

func TestCompute_DORA(t *testing.T) {
	src := &fakeSource{}
	rp := repo.Repo{
		ID: "id", Name: repo.FullName{Owner: "o", Name: "r"},
		DefaultBranch: "main", HotfixLabel: "hotfix", IncidentLabel: "incident",
	}
	w, _ := ParseWindow("2026-05", "", now)

	got, err := Compute(context.Background(), src, rp, w, now)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if got.DORA == nil {
		t.Fatal("DORA section missing")
	}
	if got.DORA.DefaultBranch != "main" || got.DORA.HotfixLabel != "hotfix" || got.DORA.IncidentLabel != "incident" {
		t.Fatalf("DORA labels: %+v", got.DORA)
	}
	if got.DORA.ChangeFailureRate != nil {
		t.Fatalf("no deployments must give a nil change failure rate, got %v", *got.DORA.ChangeFailureRate)
	}
	if len(src.doraBranches) != 1 || src.doraBranches[0] != "main" {
		t.Fatalf("DORA queried with %v", src.doraBranches)
	}
}

// TestElapsedEnd asserts a window still in progress is measured up to
// now, and a finished window keeps its end.
func TestElapsedEnd(t *testing.T) {
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	if got := ElapsedEnd(to, now); !got.Equal(now) {
		t.Fatalf("in-progress window: %v, want %v", got, now)
	}
	later := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	if got := ElapsedEnd(to, later); !got.Equal(to) {
		t.Fatalf("finished window: %v, want %v", got, to)
	}
}
