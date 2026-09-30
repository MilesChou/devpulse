package persistence_test

import (
	"context"
	"math"
	"slices"
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

	total, failed, rate, err := m.BuildFailureRate(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("BuildFailureRate: %v", err)
	}
	if total != 0 || failed != 0 || rate != 0 {
		t.Fatalf("BuildFailureRate on empty: %d/%d rate=%v", failed, total, rate)
	}

	avg, err := m.AverageBuildsPerPR(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("AverageBuildsPerPR: %v", err)
	}
	if avg != 0 {
		t.Fatalf("AverageBuildsPerPR on empty: %v", avg)
	}

	count, avgH, p50, p90, err := m.PRLeadTime(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("PRLeadTime: %v", err)
	}
	if count != 0 || avgH != 0 || p50 != 0 || p90 != 0 {
		t.Fatalf("PRLeadTime on empty: count=%d", count)
	}

	in, err := m.DORAInput(ctx, r.ID, "main", metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("DORAInput: %v", err)
	}
	if len(in.Deployments) != 0 || len(in.Incidents) != 0 {
		t.Fatalf("DORAInput on empty: %+v", in)
	}

	dist, err := m.PRSizeDistribution(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("PRSizeDistribution: %v", err)
	}
	if len(dist) != 0 {
		t.Fatalf("PRSizeDistribution on empty: %v", dist)
	}

	rwCount, rwAvg, err := m.ReviewWaitTime(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("ReviewWaitTime: %v", err)
	}
	if rwCount != 0 || rwAvg != 0 {
		t.Fatalf("ReviewWaitTime on empty: count=%d avg=%v", rwCount, rwAvg)
	}

	days, err := m.DailyBuildDuration(ctx, []string{r.ID}, metricsFrom, metricsTo)
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

	total, failed, rate, err := m.BuildFailureRate(ctx, []string{r.ID}, metricsFrom, metricsTo)
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
	avg, err := m.AverageBuildsPerPR(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("AverageBuildsPerPR: %v", err)
	}
	if math.Abs(avg-1.5) > 1e-9 {
		t.Fatalf("AverageBuildsPerPR: got %v, want 1.5", avg)
	}

	days, err := m.DailyBuildDuration(ctx, []string{r.ID}, metricsFrom, metricsTo)
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
	// Review wait reads review rows: bob reviews each PR at its
	// first_review_at.
	rvp := persistence.NewReviewPersister(p)
	for _, pr := range prs {
		if err := rvp.Upsert(ctx, pr.ID, pullrequest.Review{
			ReviewerAccount: "bob", State: pullrequest.ReviewStateCommented, SubmittedAt: *pr.FirstReviewAt,
		}); err != nil {
			t.Fatalf("seed review: %v", err)
		}
	}

	count, avgH, p50, p90, err := m.PRLeadTime(ctx, []string{r.ID}, metricsFrom, metricsTo)
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

	dist, err := m.PRSizeDistribution(ctx, []string{r.ID}, metricsFrom, metricsTo)
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
	rwCount, rwAvg, err := m.ReviewWaitTime(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("ReviewWaitTime: %v", err)
	}
	if rwCount != 3 || math.Abs(rwAvg-2) > 1e-6 {
		t.Fatalf("ReviewWaitTime: count=%d avg=%v, want 3 / 2h", rwCount, rwAvg)
	}
}

// TestMetricsPersister_ExcludesBots seeds human and bot work side by
// side and asserts the bot's PRs, builds, and reviews never count.
func TestMetricsPersister_ExcludesBots(t *testing.T) {
	p := setup(t)
	ctx := context.Background()
	r, err := persistence.NewRepoPersister(p).EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}
	seedPeopleFixture(t, p, r.ID)
	m := persistence.NewMetricsPersister(p)

	// PRs: alice #1 (10h), Bob #2 (20h), dependabot[bot] #3 (1h).
	count, avg, _, _, err := m.PRLeadTime(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("lead time: %v", err)
	}
	if count != 2 || math.Abs(avg-15) > 1e-9 {
		t.Fatalf("lead time: count=%d avg=%v, want 2 / 15h (bot PR excluded)", count, avg)
	}

	dist, _ := m.PRSizeDistribution(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if dist["XS"] != 2 {
		t.Fatalf("size dist: %v, want XS:2 (bot PR excluded)", dist)
	}

	// Builds: #1 pass, #2 fail, bot #3 fail, and an unlinked build with
	// no author (NULL owner) that passed. The NULL-owner build still
	// counts; the bot's does not.
	total, failed, _, err := m.BuildFailureRate(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("failure rate: %v", err)
	}
	if total != 3 || failed != 1 {
		t.Fatalf("failure rate: %d/%d, want 1/3", failed, total)
	}

	// Reviews: Copilot reviews alice's PR after 1 minute, bob after 2h.
	// Only the human review counts, so the wait is 2h, not ~0.
	rwCount, rwAvg, err := m.ReviewWaitTime(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("review wait: %v", err)
	}
	if rwCount != 1 || math.Abs(rwAvg-2) > 1e-9 {
		t.Fatalf("review wait: count=%d avg=%v, want 1 / 2h", rwCount, rwAvg)
	}

	authors, err := m.Authors(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("authors: %v", err)
	}
	if !slices.Equal(authors, []string{"alice", "bob"}) {
		t.Fatalf("authors: %v, want [alice bob] (normalized, bot excluded)", authors)
	}
}

// TestMetricsPersister_Scoped limits metrics to one person's accounts.
func TestMetricsPersister_Scoped(t *testing.T) {
	p := setup(t)
	ctx := context.Background()
	r, _ := persistence.NewRepoPersister(p).EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	seedPeopleFixture(t, p, r.ID)
	m := persistence.NewMetricsPersister(p)

	bob := m.Scoped([]string{"bob"})
	count, avg, _, _, err := bob.PRLeadTime(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if err != nil || count != 1 || math.Abs(avg-20) > 1e-9 {
		t.Fatalf("bob lead time: count=%d avg=%v err=%v, want 1 / 20h", count, avg, err)
	}
	// Bob's PR #2 had one failing build; the matching is case-insensitive
	// (author stored as "Bob").
	total, failed, _, _ := bob.BuildFailureRate(ctx, []string{r.ID}, metricsFrom, metricsTo)
	if total != 1 || failed != 1 {
		t.Fatalf("bob failure rate: %d/%d, want 1/1", failed, total)
	}
	if n, _, _ := bob.ReviewWaitTime(ctx, []string{r.ID}, metricsFrom, metricsTo); n != 0 {
		t.Fatalf("bob review wait count: %d, want 0 (his PR has no human review)", n)
	}

	// A scope that names the bot still excludes it.
	botScope := m.Scoped([]string{"dependabot"})
	if count, _, _, _, _ := botScope.PRLeadTime(ctx, []string{r.ID}, metricsFrom, metricsTo); count != 0 {
		t.Fatalf("bot scope: count=%d, want 0", count)
	}

	// A member without accounts matches nothing.
	none := m.Scoped([]string{})
	if total, _, _, _ := none.BuildFailureRate(ctx, []string{r.ID}, metricsFrom, metricsTo); total != 0 {
		t.Fatalf("empty scope: total=%d, want 0", total)
	}
}

// seedPeopleFixture writes PRs, builds, and reviews by two humans
// (alice, "Bob" with a capital) and dependabot, all in the metrics window.
func seedPeopleFixture(t *testing.T, p *persistence.Persister, repoID string) {
	t.Helper()
	ctx := context.Background()

	mk := func(number int, author string, leadHours float64) pullrequest.PullRequest {
		created := metricsFrom.Add(time.Duration(number) * 24 * time.Hour)
		ready := created
		merged := created.Add(time.Duration(leadHours * float64(time.Hour)))
		return pullrequest.PullRequest{
			RepoID: repoID, Number: number, Author: author, Status: pullrequest.StatusMerged,
			Additions: 10, TotalChangedLines: 10, SizeBucket: pullrequest.SizeBucket(10),
			CreatedAt: created, ReadyAt: &ready, MergedAt: &merged,
		}
	}
	prs := []pullrequest.PullRequest{mk(1, "alice", 10), mk(2, "Bob", 20), mk(3, "dependabot[bot]", 1)}
	if _, err := persistence.NewPullRequestPersister(p).UpsertMany(ctx, prs); err != nil {
		t.Fatalf("seed prs: %v", err)
	}

	rvp := persistence.NewReviewPersister(p)
	reviews := []struct {
		pr       int
		reviewer string
		after    time.Duration
	}{
		{0, "copilot-pull-request-reviewer", time.Minute},
		{0, "bob", 2 * time.Hour},
		{1, "copilot-pull-request-reviewer", time.Minute},
	}
	for _, rv := range reviews {
		at := prs[rv.pr].ReadyAt.Add(rv.after)
		if err := rvp.Upsert(ctx, prs[rv.pr].ID, pullrequest.Review{
			ReviewerAccount: rv.reviewer, State: pullrequest.ReviewStateCommented, SubmittedAt: at,
		}); err != nil {
			t.Fatalf("seed review: %v", err)
		}
	}

	sha, _ := commitsha.Parse("bbb1234567890abcdef1234567890abcdef12345")
	at := func(n int) time.Time { return metricsFrom.Add(time.Duration(n)*24*time.Hour + time.Hour) }
	builds := []build.Build{
		{ExternalID: "1", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 1, Status: build.StatusPassed, StartedAt: at(1)},
		{ExternalID: "2", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 2, Status: build.StatusFailed, StartedAt: at(2)},
		{ExternalID: "3", CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 3, Status: build.StatusFailed, StartedAt: at(3)},
		// Not linked to a PR and no known author: owner is NULL.
		{ExternalID: "4", CommitSHA: sha, Trigger: build.TriggerPullRequest, Status: build.StatusPassed, StartedAt: at(4)},
	}
	if _, err := persistence.NewBuildPersister(p).UpsertMany(ctx, repoID, "github-actions", builds); err != nil {
		t.Fatalf("seed builds: %v", err)
	}
}

// TestMetricsPersister_AcrossRepos pools two repos. Lead times of 1 h
// and 3 h in repo A and 100 h in repo B give avg 34.67 h and p50 3 h
// across both, not the average of the two repos' p50s. Both repos have
// a PR #1, so builds per PR must count them as two PRs.
func TestMetricsPersister_AcrossRepos(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	bp := persistence.NewBuildPersister(p)
	m := persistence.NewMetricsPersister(p)
	ctx := context.Background()

	a, err := rp.EnsureID(ctx, "github", mustFullName(t, "acme/a"))
	if err != nil {
		t.Fatalf("ensure a: %v", err)
	}
	b, err := rp.EnsureID(ctx, "github", mustFullName(t, "acme/b"))
	if err != nil {
		t.Fatalf("ensure b: %v", err)
	}

	mkPR := func(repoID string, number int, leadHours float64) pullrequest.PullRequest {
		created := metricsFrom.Add(time.Duration(number) * time.Hour)
		merged := created.Add(time.Duration(leadHours * float64(time.Hour)))
		return pullrequest.PullRequest{
			RepoID: repoID, Number: number, Author: "alice", Status: pullrequest.StatusMerged,
			Additions: 10, TotalChangedLines: 10, SizeBucket: pullrequest.SizeBucket(10),
			CreatedAt: created, MergedAt: &merged,
		}
	}
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{
		mkPR(a.ID, 1, 1), mkPR(a.ID, 2, 3), mkPR(b.ID, 1, 100),
	}); err != nil {
		t.Fatalf("seed prs: %v", err)
	}

	sha, err := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	if err != nil {
		t.Fatalf("parse sha: %v", err)
	}
	start := metricsFrom.Add(time.Hour)
	prBuild := func(id string) build.Build {
		return build.Build{ExternalID: id, CommitSHA: sha, Trigger: build.TriggerPullRequest, PRNumber: 1,
			Status: build.StatusPassed, StartedAt: start}
	}
	// PR #1 of repo A: 2 builds. PR #1 of repo B: 1 build.
	if _, err := bp.UpsertMany(ctx, a.ID, "github-actions", []build.Build{prBuild("a1"), prBuild("a2")}); err != nil {
		t.Fatalf("seed builds a: %v", err)
	}
	if _, err := bp.UpsertMany(ctx, b.ID, "github-actions", []build.Build{prBuild("b1")}); err != nil {
		t.Fatalf("seed builds b: %v", err)
	}

	both := []string{a.ID, b.ID}
	count, avgH, p50, _, err := m.PRLeadTime(ctx, both, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("PRLeadTime: %v", err)
	}
	if count != 3 || math.Abs(avgH-104.0/3.0) > 1e-9 || math.Abs(p50-3) > 1e-9 {
		t.Fatalf("PRLeadTime across repos: count=%d avg=%v p50=%v, want 3, 34.67, 3", count, avgH, p50)
	}

	// A single repo stays exactly what it was.
	count, avgH, _, _, err = m.PRLeadTime(ctx, []string{a.ID}, metricsFrom, metricsTo)
	if err != nil || count != 2 || math.Abs(avgH-2) > 1e-9 {
		t.Fatalf("PRLeadTime repo A: count=%d avg=%v err=%v", count, avgH, err)
	}

	avg, err := m.AverageBuildsPerPR(ctx, both, metricsFrom, metricsTo)
	if err != nil {
		t.Fatalf("AverageBuildsPerPR: %v", err)
	}
	if math.Abs(avg-1.5) > 1e-9 {
		t.Fatalf("AverageBuildsPerPR across repos: got %v, want 1.5 (two PRs #1, 3 builds)", avg)
	}

	if _, _, _, _, err := m.PRLeadTime(ctx, nil, metricsFrom, metricsTo); err != nil {
		t.Fatalf("empty repo set: %v", err)
	}
}
