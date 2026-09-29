package fetching_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

func openPR(repoID string, number int) pullrequest.PullRequest {
	pr := makePR(repoID, number)
	pr.Status = pullrequest.StatusOpen
	pr.MergedAt = nil
	return pr
}

// TestBackfill_FetchesFirstCommitOnlyForMerged asserts the lead-time
// start is fetched for merged PRs and skipped for open ones.
func TestBackfill_FetchesFirstCommitOnlyForMerged(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	first := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)

	vcs := &fakeVCSProvider{
		latestNumber:  2,
		prs:           map[int]pullrequest.PullRequest{1: makePR(r.ID, 1), 2: openPR(r.ID, 2)},
		firstCommitAt: map[int]time.Time{1: first, 2: first},
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	if _, err := orch.BackfillPullRequestsByNumber(context.Background(), r); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	merged, _ := pp.FindByNumber(context.Background(), r.ID, 1)
	if merged.FirstCommitAt == nil || !merged.FirstCommitAt.Equal(first) {
		t.Fatalf("merged PR first_commit_at: %v", merged.FirstCommitAt)
	}
	open, _ := pp.FindByNumber(context.Background(), r.ID, 2)
	if open.FirstCommitAt != nil {
		t.Fatalf("open PR should not carry first_commit_at: %v", open.FirstCommitAt)
	}
}

// TestBackfill_FirstCommitErrorDoesNotStall asserts a first-commit fetch
// failure does not stop the backfill: the PR is written without
// first_commit_at, the next number is still synced, and the row is left
// for CompletePullRequestFacts to retry.
func TestBackfill_FirstCommitErrorDoesNotStall(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()
	vcs := &fakeVCSProvider{
		latestNumber:   2,
		prs:            map[int]pullrequest.PullRequest{1: makePR(r.ID, 1), 2: makePR(r.ID, 2)},
		firstCommitErr: errors.New("boom"),
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil || written != 2 {
		t.Fatalf("want 2 written and no error, got written=%d err=%v", written, err)
	}
	pr, _ := pp.FindByNumber(ctx, r.ID, 1)
	if pr.FirstCommitAt != nil {
		t.Fatalf("first_commit_at should be empty after a failed fetch: %v", pr.FirstCommitAt)
	}

	// The upstream call recovers; the completion pass fills the gap.
	first := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)
	vcs.firstCommitErr = nil
	vcs.firstCommitAt = map[int]time.Time{1: first, 2: first}
	completed, err := orch.CompletePullRequestFacts(ctx, r)
	if err != nil || completed != 2 {
		t.Fatalf("complete: completed=%d err=%v", completed, err)
	}
	pr, _ = pp.FindByNumber(ctx, r.ID, 1)
	if pr.FirstCommitAt == nil || !pr.FirstCommitAt.Equal(first) {
		t.Fatalf("first_commit_at after completion: %v", pr.FirstCommitAt)
	}
}

// TestBackfill_ClampsFirstCommitAfterMerge asserts an author date later
// than the merge (clock skew) is stored as the merge time, which keeps
// it inside the range every DB dialect can store.
func TestBackfill_ClampsFirstCommitAfterMerge(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()
	pr1 := makePR(r.ID, 1)
	vcs := &fakeVCSProvider{
		latestNumber:  1,
		prs:           map[int]pullrequest.PullRequest{1: pr1},
		firstCommitAt: map[int]time.Time{1: time.Date(2106, 2, 7, 0, 0, 0, 0, time.UTC)},
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	if _, err := orch.BackfillPullRequestsByNumber(ctx, r); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	got, _ := pp.FindByNumber(ctx, r.ID, 1)
	if got.FirstCommitAt == nil || !got.FirstCommitAt.Equal(*pr1.MergedAt) {
		t.Fatalf("first_commit_at: %v, want merge time %v", got.FirstCommitAt, pr1.MergedAt)
	}
}

// TestCompletePullRequestFacts_FillsPreDORARows covers an upgraded store:
// rows written before the DORA columns existed have no base_ref, and
// neither the backfill nor the refresh revisits them. The completion
// pass re-syncs them, skips complete rows and numbers below the sync
// floor, and stops at the first failure so the next sync resumes.
func TestCompletePullRequestFacts_FillsPreDORARows(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()
	first := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)

	withBase := func(pr pullrequest.PullRequest) pullrequest.PullRequest {
		pr.BaseRef, pr.HeadRef = "main", fmt.Sprintf("feature-%d", pr.Number)
		return pr
	}

	// Seed: #1..#3 as a pre-DORA sync left them (no base_ref, no first
	// commit); #4 complete.
	complete := withBase(makePR(r.ID, 4))
	complete.FirstCommitAt = &first
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{
		makePR(r.ID, 1), makePR(r.ID, 2), makePR(r.ID, 3), complete,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	vcs := &fakeVCSProvider{
		prs: map[int]pullrequest.PullRequest{
			1: withBase(makePR(r.ID, 1)),
			2: withBase(makePR(r.ID, 2)),
			3: withBase(makePR(r.ID, 3)),
			4: complete,
		},
		getErrFor:     map[int]error{3: errors.New("rate limited")},
		firstCommitAt: map[int]time.Time{1: first, 2: first, 3: first},
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	r.PRSyncStartNumber = 2
	completed, err := orch.CompletePullRequestFacts(ctx, r)
	if err == nil {
		t.Fatal("want the #3 failure to be returned")
	}
	if completed != 1 {
		t.Fatalf("completed: %d, want 1", completed)
	}
	if len(vcs.gotNumbers) != 2 || vcs.gotNumbers[0] != 2 || vcs.gotNumbers[1] != 3 {
		t.Fatalf("visited %v, want [2 3]", vcs.gotNumbers)
	}
	pr2, _ := pp.FindByNumber(ctx, r.ID, 2)
	if pr2.BaseRef != "main" || pr2.FirstCommitAt == nil {
		t.Fatalf("#2 not completed: base=%q first=%v", pr2.BaseRef, pr2.FirstCommitAt)
	}

	// Next sync resumes at #3.
	delete(vcs.getErrFor, 3)
	vcs.gotNumbers = nil
	completed, err = orch.CompletePullRequestFacts(ctx, r)
	if err != nil || completed != 1 {
		t.Fatalf("resume: completed=%d err=%v", completed, err)
	}
	if len(vcs.gotNumbers) != 1 || vcs.gotNumbers[0] != 3 {
		t.Fatalf("resume visited %v, want [3]", vcs.gotNumbers)
	}
}

// upstreamPR returns a PR as the upstream reports it at updatedAt.
func upstreamPR(pr pullrequest.PullRequest, updatedAt time.Time) pullrequest.PullRequest {
	pr.SourceUpdatedAt = updatedAt
	return pr
}

// TestRefreshPullRequests_FirstRunRecordsLaterMerge covers the spec
// scenario "PR merged after the first sync" on the first run, before
// the repo has a watermark: the listing only seeds the watermark, every
// stored open PR is refreshed, and the backfill cursor has moved past
// #7 so only this pass can observe its merge.
func TestRefreshPullRequests_FirstRunRecordsLaterMerge(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()
	first := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)

	vcs := &fakeVCSProvider{
		latestNumber: 8,
		prs: map[int]pullrequest.PullRequest{
			7: openPR(r.ID, 7),
			8: openPR(r.ID, 8),
		},
		firstCommitAt: map[int]time.Time{7: first},
		updatedNewest: newest,
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	if _, err := orch.BackfillPullRequestsByNumber(ctx, r); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Upstream: #7 merged, #8 was deleted (404).
	vcs.prs = map[int]pullrequest.PullRequest{7: makePR(r.ID, 7)}
	vcs.gotNumbers = nil

	got, err := orch.RefreshPullRequests(ctx, r)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got.Refreshed != 1 {
		t.Fatalf("refreshed: %d", got.Refreshed)
	}
	if len(vcs.gotNumbers) != 2 || vcs.gotNumbers[0] != 7 || vcs.gotNumbers[1] != 8 {
		t.Fatalf("refresh visited %v, want [7 8]", vcs.gotNumbers)
	}
	if vcs.gotSince == nil || !vcs.gotSince.IsZero() {
		t.Fatalf("first run must list with a zero since, got %v", vcs.gotSince)
	}
	// A 404 counts as done, so the watermark still advances.
	if got.Watermark == nil || !got.Watermark.Equal(newest) {
		t.Fatalf("watermark: %v, want %v", got.Watermark, newest)
	}

	pr, _ := pp.FindByNumber(ctx, r.ID, 7)
	if pr.Status != pullrequest.StatusMerged || pr.MergedAt == nil {
		t.Fatalf("#7 not merged after refresh: %+v", pr)
	}
	if pr.FirstCommitAt == nil || !pr.FirstCommitAt.Equal(first) {
		t.Fatalf("#7 first_commit_at: %v", pr.FirstCommitAt)
	}
}

// TestRefreshPullRequests_ListedAndOpenPRs covers both refresh sets:
//   - #2, stored as closed, was reopened and merged: only the listing
//     since the watermark can reveal it.
//   - #4, stored as open, is not listed (its updated_at is behind the
//     watermark, as with a stale cached copy the backfill wrote) but is
//     merged upstream: the open set heals it.
//
// It also asserts the number filters: #9 is above the stored max (the
// backfill's job) and #1 is below the floor.
func TestRefreshPullRequests_ListedAndOpenPRs(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	closed := openPR(r.ID, 2)
	closed.Status = pullrequest.StatusClosed
	closedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	closed.ClosedAt = &closedAt
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{
		openPR(r.ID, 1), closed, makePR(r.ID, 3), openPR(r.ID, 4),
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	watermark := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	listed := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	r.PRUpdatedWatermark = &watermark
	r.PRSyncStartNumber = 2

	vcs := &fakeVCSProvider{
		prs: map[int]pullrequest.PullRequest{
			2: upstreamPR(makePR(r.ID, 2), listed),
			4: makePR(r.ID, 4),
		},
		updated: []pullrequest.Stamp{
			{Number: 9, UpdatedAt: newest},
			{Number: 2, UpdatedAt: listed},
			{Number: 1, UpdatedAt: listed},
		},
		updatedNewest: newest,
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	got, err := orch.RefreshPullRequests(ctx, r)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if vcs.gotSince == nil || !vcs.gotSince.Equal(watermark) {
		t.Fatalf("listed since %v, want %v", vcs.gotSince, watermark)
	}
	if len(vcs.gotNumbers) != 2 || vcs.gotNumbers[0] != 2 || vcs.gotNumbers[1] != 4 {
		t.Fatalf("refresh visited %v, want [2 4]", vcs.gotNumbers)
	}
	if got.Refreshed != 2 || got.Watermark == nil || !got.Watermark.Equal(newest) {
		t.Fatalf("got %+v, want 2 refreshed and watermark %v", got, newest)
	}
	for _, n := range []int{2, 4} {
		pr, _ := pp.FindByNumber(ctx, r.ID, n)
		if pr.Status != pullrequest.StatusMerged || pr.MergedAt == nil {
			t.Fatalf("#%d not merged after refresh: %+v", n, pr)
		}
	}
}

// TestRefreshPullRequests_StaleDetailHoldsWatermark covers a cached PR
// detail that is older than the listing: it must not be written, and
// the watermark must not move past the PR.
func TestRefreshPullRequests_StaleDetailHoldsWatermark(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	closed := openPR(r.ID, 2)
	closed.Status = pullrequest.StatusClosed
	closedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	closed.ClosedAt = &closedAt
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{closed}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	watermark := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	listed := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	r.PRUpdatedWatermark = &watermark

	// The detail is merged but older than its listing: a cached copy.
	vcs := &fakeVCSProvider{
		prs:           map[int]pullrequest.PullRequest{2: upstreamPR(makePR(r.ID, 2), listed.Add(-time.Hour))},
		updated:       []pullrequest.Stamp{{Number: 2, UpdatedAt: listed}},
		updatedNewest: listed,
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	got, err := orch.RefreshPullRequests(ctx, r)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got.Refreshed != 0 || got.Watermark != nil {
		t.Fatalf("got %+v, want nothing refreshed and no watermark", got)
	}
	pr, _ := pp.FindByNumber(ctx, r.ID, 2)
	if pr.Status != pullrequest.StatusClosed {
		t.Fatalf("stale detail was written: %+v", pr)
	}
}

// TestRefreshPullRequests_ErrorHoldsWatermark asserts one failing PR is
// skipped, the rest are still refreshed, and the watermark does not
// advance so the failed PR is listed again next sync.
func TestRefreshPullRequests_ErrorHoldsWatermark(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{openPR(r.ID, 1), openPR(r.ID, 2)}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	watermark := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	listed := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	r.PRUpdatedWatermark = &watermark
	vcs := &fakeVCSProvider{
		prs:       map[int]pullrequest.PullRequest{2: upstreamPR(makePR(r.ID, 2), listed)},
		getErrFor: map[int]error{1: errors.New("503")},
		updated: []pullrequest.Stamp{
			{Number: 2, UpdatedAt: listed},
			{Number: 1, UpdatedAt: listed},
		},
		updatedNewest: listed,
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	got, err := orch.RefreshPullRequests(ctx, r)
	if err != nil || got.Refreshed != 1 {
		t.Fatalf("want (1, nil), got (%d, %v)", got.Refreshed, err)
	}
	if got.Watermark != nil {
		t.Fatalf("watermark advanced to %v despite a failed PR", got.Watermark)
	}
}

// TestRefreshPullRequests_ListErrorAborts asserts a listing failure
// aborts the pass without touching any PR.
func TestRefreshPullRequests_ListErrorAborts(t *testing.T) {
	p, r := setup(t)
	vcs := &fakeVCSProvider{updatedErr: errors.New("503")}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p),
		persistence.NewPullRequestPersister(p), persistence.NewReviewPersister(p), nil, nil)

	got, err := orch.RefreshPullRequests(context.Background(), r)
	if err == nil || got.Watermark != nil || len(vcs.gotNumbers) != 0 {
		t.Fatalf("want error and no work, got %+v err=%v visited=%v", got, err, vcs.gotNumbers)
	}
}

// TestRefreshPullRequests_NoUpstreamPRsKeepsWatermark asserts a repo with
// no PRs upstream yields no watermark rather than a zero one.
func TestRefreshPullRequests_NoUpstreamPRsKeepsWatermark(t *testing.T) {
	p, r := setup(t)
	orch := fetching.NewOrchestrator(nil, &fakeVCSProvider{}, persistence.NewBuildPersister(p),
		persistence.NewPullRequestPersister(p), persistence.NewReviewPersister(p), nil, nil)

	got, err := orch.RefreshPullRequests(context.Background(), r)
	if err != nil || got.Refreshed != 0 || got.Watermark != nil {
		t.Fatalf("got %+v err=%v, want zero result", got, err)
	}
}

// TestSyncIncidents_MirrorsWithRepoLabel asserts the repo's label is
// passed upstream and the stored set mirrors the result.
func TestSyncIncidents_MirrorsWithRepoLabel(t *testing.T) {
	p, r := setup(t)
	ip := persistence.NewIncidentPersister(p)
	ctx := context.Background()
	r.IncidentLabel = "sev-1"

	opened := time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC)
	closed := opened.Add(3 * time.Hour)
	vcs := &fakeVCSProvider{incidents: []incident.Incident{
		{Source: incident.SourceGitHubIssue, Number: 1, Title: "open", OpenedAt: opened},
		{Source: incident.SourceGitHubIssue, Number: 2, Title: "closed", OpenedAt: opened, ResolvedAt: &closed},
	}}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p),
		persistence.NewPullRequestPersister(p), persistence.NewReviewPersister(p), ip, nil)

	n, err := orch.SyncIncidents(ctx, r)
	if err != nil || n != 2 {
		t.Fatalf("want (2, nil), got (%d, %v)", n, err)
	}
	if vcs.gotLabel != "sev-1" {
		t.Fatalf("label: %q", vcs.gotLabel)
	}
	got, _ := ip.List(ctx, r.ID)
	if len(got) != 2 || got[0].ResolvedAt != nil || got[1].ResolvedAt == nil {
		t.Fatalf("stored: %+v", got)
	}

	// An upstream failure must leave the stored set untouched.
	vcs.incidentsErr = errors.New("503")
	if _, err := orch.SyncIncidents(ctx, r); err == nil {
		t.Fatal("want error")
	}
	if got, _ := ip.List(ctx, r.ID); len(got) != 2 {
		t.Fatalf("stored set changed on failure: %+v", got)
	}
}

// TestFetchAllBuilds_LinksBuildsToPullRequests asserts the build sync
// fills pr_number from the stored PRs when the CI provider returns
// none, which is what GitHub Actions does in practice.
func TestFetchAllBuilds_LinksBuildsToPullRequests(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	pr := openPR(r.ID, 5)
	pr.HeadRef = "feature/login"
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{pr}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	ci := &fakeCIProvider{builds: []build.Build{{
		ExternalID: "1", CommitSHA: sha, Status: build.StatusPassed,
		Trigger: build.TriggerPullRequest, Branch: "feature/login",
		StartedAt: pr.CreatedAt.Add(time.Hour),
	}}}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{ci}, &fakeVCSProvider{},
		persistence.NewBuildPersister(p), pp, persistence.NewReviewPersister(p), nil, nil)

	if _, err := orch.FetchAllBuilds(ctx, r); err != nil {
		t.Fatalf("fetch builds: %v", err)
	}
	var got int
	if err := p.QueryRowCtx(ctx, `SELECT pr_number FROM builds WHERE external_id = '1'`).Scan(&got); err != nil {
		t.Fatalf("read build: %v", err)
	}
	if got != 5 {
		t.Fatalf("pr_number: %d, want 5", got)
	}
}
