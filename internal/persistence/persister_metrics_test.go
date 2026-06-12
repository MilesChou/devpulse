package persistence_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// metricsWindow is the [from, to) month window every metrics test uses.
var metricsFrom = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
var metricsTo = metricsFrom.AddDate(0, 1, 0)

// TestMetricsPersister_EmptyStore runs every metrics query against an
// empty store. Beyond the zero-value contract, this is the dialect
// smoke test: the CI matrix replays it on PostgreSQL and MySQL, so any
// SQL that only SQLite accepts (e.g. an alias-less derived table) fails
// here instead of at `devpulse metrics` runtime.
func TestMetricsPersister_EmptyStore(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	m := persistence.NewMetricsPersister(p)
	ctx := context.Background()

	r, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}

	total, failed, rate, err := m.BuildFailureRate(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("BuildFailureRate: %v", err)
	}
	if total != 0 || failed != 0 || rate != 0 {
		t.Fatalf("BuildFailureRate on empty: %d/%d rate=%v", failed, total, rate)
	}

	avg, err := m.AverageBuildsPerPR(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("AverageBuildsPerPR: %v", err)
	}
	if avg != 0 {
		t.Fatalf("AverageBuildsPerPR on empty: %v", avg)
	}

	count, avgH, p50, p90, err := m.PRLeadTime(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("PRLeadTime: %v", err)
	}
	if count != 0 || avgH != 0 || p50 != 0 || p90 != 0 {
		t.Fatalf("PRLeadTime on empty: count=%d", count)
	}

	dist, err := m.PRSizeDistribution(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("PRSizeDistribution: %v", err)
	}
	if len(dist) != 0 {
		t.Fatalf("PRSizeDistribution on empty: %v", dist)
	}

	rwCount, rwAvg, err := m.ReviewWaitTime(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("ReviewWaitTime: %v", err)
	}
	if rwCount != 0 || rwAvg != 0 {
		t.Fatalf("ReviewWaitTime on empty: count=%d avg=%v", rwCount, rwAvg)
	}

	days, err := m.DailyBuildDuration(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("DailyBuildDuration: %v", err)
	}
	if len(days) != 0 {
		t.Fatalf("DailyBuildDuration on empty: %v", days)
	}
}

func TestMetricsPersister_BuildMetrics(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	bp := persistence.NewBuildPersister(p)
	m := persistence.NewMetricsPersister(p)
	ctx := context.Background()

	r, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}

	day1 := metricsFrom.Add(10 * time.Hour)
	day2 := metricsFrom.AddDate(0, 0, 1).Add(10 * time.Hour)
	finished := func(start time.Time, d time.Duration) *time.Time {
		f := start.Add(d)
		return &f
	}

	sha, err := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	if err != nil {
		t.Fatalf("parse sha: %v", err)
	}
	seed := []build.Build{
		// PR #1: two PR builds, one failed → counted in failure rate.
		{ExternalID: "1", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 1,
			Status: build.StatusPassed, StartedAt: day1, FinishedAt: finished(day1, 60*time.Second)},
		{ExternalID: "2", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 1,
			Status: build.StatusFailed, StartedAt: day1.Add(time.Hour), FinishedAt: finished(day1.Add(time.Hour), 120*time.Second)},
		// PR #2: one errored PR build → IsFailure, counted.
		{ExternalID: "3", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 2,
			Status: build.StatusErrored, StartedAt: day2, FinishedAt: finished(day2, 30*time.Second)},
		// Push build: excluded from failure rate (is_pull_request=false)
		// but contributes to DailyBuildDuration.
		{ExternalID: "4", CommitSHA: sha, Trigger: build.TriggerPush,
			Status: build.StatusFailed, StartedAt: day2.Add(time.Hour), FinishedAt: finished(day2.Add(time.Hour), 90*time.Second)},
		// Outside the window: ignored everywhere.
		{ExternalID: "5", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 9,
			Status: build.StatusFailed, StartedAt: metricsTo.Add(time.Hour)},
	}
	if _, err := bp.UpsertMany(ctx, r.ID, "github-actions", seed); err != nil {
		t.Fatalf("seed builds: %v", err)
	}

	total, failed, rate, err := m.BuildFailureRate(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("BuildFailureRate: %v", err)
	}
	if total != 3 || failed != 2 {
		t.Fatalf("BuildFailureRate: got %d/%d, want 2/3", failed, total)
	}
	if math.Abs(rate-2.0/3.0) > 1e-9 {
		t.Fatalf("rate: got %v", rate)
	}

	// PR #1 has 2 builds, PR #2 has 1 → avg 1.5. The push build (no
	// pr_number) and out-of-window build are excluded.
	avg, err := m.AverageBuildsPerPR(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("AverageBuildsPerPR: %v", err)
	}
	if math.Abs(avg-1.5) > 1e-9 {
		t.Fatalf("AverageBuildsPerPR: got %v, want 1.5", avg)
	}

	days, err := m.DailyBuildDuration(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("DailyBuildDuration: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("DailyBuildDuration: got %d days, want 2 (%v)", len(days), days)
	}
	// Day 1: (60+120)/2 = 90s over 2 builds. Day 2: (30+90)/2 = 60s.
	if days[0].Count != 2 || math.Abs(days[0].AvgSeconds-90) > 1e-9 {
		t.Fatalf("day1: %+v", days[0])
	}
	if days[1].Count != 2 || math.Abs(days[1].AvgSeconds-60) > 1e-9 {
		t.Fatalf("day2: %+v", days[1])
	}
}

func TestMetricsPersister_PRMetrics(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	m := persistence.NewMetricsPersister(p)
	ctx := context.Background()

	r, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}

	mkPR := func(number int, leadHours float64, lines int) pullrequest.PullRequest {
		created := metricsFrom.Add(time.Duration(number) * 24 * time.Hour)
		ready := created.Add(30 * time.Minute)
		firstReview := ready.Add(2 * time.Hour)
		merged := created.Add(time.Duration(leadHours * float64(time.Hour)))
		return pullrequest.PullRequest{
			RepoID:            r.ID,
			Number:            number,
			Author:            "alice",
			Status:            pullrequest.StatusMerged,
			Additions:         lines,
			TotalChangedLines: lines,
			SizeBucket:        pullrequest.SizeBucket(lines),
			CreatedAt:         created,
			ReadyAt:           &ready,
			FirstReviewAt:     &firstReview,
			MergedAt:          &merged,
		}
	}

	prs := []pullrequest.PullRequest{
		mkPR(1, 10, 10),  // XS
		mkPR(2, 20, 100), // S
		mkPR(3, 30, 600), // L
	}
	if _, err := pp.UpsertMany(ctx, prs); err != nil {
		t.Fatalf("seed prs: %v", err)
	}

	count, avgH, p50, p90, err := m.PRLeadTime(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("PRLeadTime: %v", err)
	}
	if count != 3 {
		t.Fatalf("PRLeadTime count: got %d, want 3", count)
	}
	if math.Abs(avgH-20) > 1e-6 {
		t.Fatalf("avg lead: got %v, want 20", avgH)
	}
	if math.Abs(p50-20) > 1e-6 {
		t.Fatalf("p50: got %v, want 20", p50)
	}
	// Linear interpolation over [10,20,30] at p90 → 28.
	if math.Abs(p90-28) > 1e-6 {
		t.Fatalf("p90: got %v, want 28", p90)
	}

	dist, err := m.PRSizeDistribution(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("PRSizeDistribution: %v", err)
	}
	want := map[string]int{"XS": 1, "S": 1, "L": 1}
	if len(dist) != len(want) {
		t.Fatalf("dist: got %v, want %v", dist, want)
	}
	for k, v := range want {
		if dist[k] != v {
			t.Fatalf("dist[%s]: got %d, want %d (%v)", k, dist[k], v, dist)
		}
	}

	// Every PR waited 2h between ready and first review.
	rwCount, rwAvg, err := m.ReviewWaitTime(ctx, r.ID, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("ReviewWaitTime: %v", err)
	}
	if rwCount != 3 || math.Abs(rwAvg-2) > 1e-6 {
		t.Fatalf("ReviewWaitTime: count=%d avg=%v, want 3 / 2h", rwCount, rwAvg)
	}
}
