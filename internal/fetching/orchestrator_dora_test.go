package fetching_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/incident"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
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

// TestBackfill_FirstCommitErrorFailsFast asserts a first-commit fetch
// failure is treated like any other per-PR upstream failure.
func TestBackfill_FirstCommitErrorFailsFast(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	vcs := &fakeVCSProvider{
		latestNumber:   1,
		prs:            map[int]pullrequest.PullRequest{1: makePR(r.ID, 1)},
		firstCommitErr: errors.New("boom"),
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	written, err := orch.BackfillPullRequestsByNumber(context.Background(), r)
	if err == nil || written != 0 {
		t.Fatalf("want error and 0 written, got written=%d err=%v", written, err)
	}
	if _, has, _ := pp.MaxNumber(context.Background(), r.ID); has {
		t.Fatal("PR row written despite first-commit failure")
	}
}

// TestRefreshOpenPullRequests_RecordsLaterMerge covers the spec scenario
// "PR merged after the first sync": the backfill cursor has moved past
// #7, so only the refresh pass can observe its merge.
func TestRefreshOpenPullRequests_RecordsLaterMerge(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()
	first := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)

	vcs := &fakeVCSProvider{
		latestNumber: 8,
		prs: map[int]pullrequest.PullRequest{
			7: openPR(r.ID, 7),
			8: openPR(r.ID, 8),
		},
		firstCommitAt: map[int]time.Time{7: first},
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	if _, err := orch.BackfillPullRequestsByNumber(ctx, r); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	// Upstream: #7 merged, #8 was deleted (404).
	vcs.prs = map[int]pullrequest.PullRequest{7: makePR(r.ID, 7)}
	vcs.gotNumbers = nil

	refreshed, err := orch.RefreshOpenPullRequests(ctx, r)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed: %d", refreshed)
	}
	if len(vcs.gotNumbers) != 2 || vcs.gotNumbers[0] != 7 || vcs.gotNumbers[1] != 8 {
		t.Fatalf("refresh visited %v, want [7 8]", vcs.gotNumbers)
	}

	got, _ := pp.FindByNumber(ctx, r.ID, 7)
	if got.Status != pullrequest.StatusMerged || got.MergedAt == nil {
		t.Fatalf("#7 not merged after refresh: %+v", got)
	}
	if got.FirstCommitAt == nil || !got.FirstCommitAt.Equal(first) {
		t.Fatalf("#7 first_commit_at: %v", got.FirstCommitAt)
	}
	open, _ := pp.ListOpenNumbers(ctx, r.ID)
	if len(open) != 1 || open[0] != 8 {
		t.Fatalf("still open: %v (the deleted #8 stays for the next retry)", open)
	}
}

// TestRefreshOpenPullRequests_ErrorDoesNotAbort asserts one failing PR
// is skipped and the rest are still refreshed.
func TestRefreshOpenPullRequests_ErrorDoesNotAbort(t *testing.T) {
	p, r := setup(t)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{openPR(r.ID, 1), openPR(r.ID, 2)}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	vcs := &fakeVCSProvider{
		prs:       map[int]pullrequest.PullRequest{2: makePR(r.ID, 2)},
		getErrFor: map[int]error{1: errors.New("503")},
	}
	orch := fetching.NewOrchestrator(nil, vcs, persistence.NewBuildPersister(p), pp,
		persistence.NewReviewPersister(p), nil, nil)

	refreshed, err := orch.RefreshOpenPullRequests(ctx, r)
	if err != nil || refreshed != 1 {
		t.Fatalf("want (1, nil), got (%d, %v)", refreshed, err)
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
