package fetching_test

import (
	"context"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/persistencetest"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// fakeCIProvider returns canned builds.
type fakeCIProvider struct {
	builds []build.Build
	err    error
}

func (f *fakeCIProvider) ListBuildsInMonth(
	_ context.Context,
	_ repo.FullName,
	_ fetching.MonthRange,
) ([]build.Build, error) {
	return f.builds, f.err
}

// fakeVCSProvider stubs out every method the orchestrator calls.
type fakeVCSProvider struct {
	pulls    []pullrequest.PullRequest
	allPulls []pullrequest.PullRequest
	detail   pullrequest.PullRequest
	reviews  []pullrequest.Review
	logins   map[commitsha.SHA]*string
	listErr  error
	getErr   error
	revsErr  error
	bulkErr  error
}

func (f *fakeVCSProvider) ListPullRequestsInMonth(
	_ context.Context, _ string, _ repo.FullName, _ fetching.MonthRange,
) ([]pullrequest.PullRequest, error) {
	return f.pulls, f.listErr
}

func (f *fakeVCSProvider) ListAllPullRequests(
	_ context.Context, _ string, _ repo.FullName,
) ([]pullrequest.PullRequest, error) {
	return f.allPulls, nil
}

func (f *fakeVCSProvider) GetPullRequest(
	_ context.Context, _ string, _ repo.FullName, number int,
) (pullrequest.PullRequest, error) {
	d := f.detail
	d.Number = number
	return d, f.getErr
}

func (f *fakeVCSProvider) ListReviews(
	_ context.Context, _ repo.FullName, _ int,
) ([]pullrequest.Review, error) {
	return f.reviews, f.revsErr
}

func (f *fakeVCSProvider) GetCommitAuthorAccountsBulk(
	_ context.Context, _ repo.FullName, shas []commitsha.SHA,
) (map[commitsha.SHA]*string, error) {
	if f.bulkErr != nil {
		return nil, f.bulkErr
	}
	out := make(map[commitsha.SHA]*string, len(shas))
	for _, s := range shas {
		out[s] = f.logins[s]
	}
	return out, nil
}

func (f *fakeVCSProvider) GetRepo(
	_ context.Context, name repo.FullName,
) (repo.Repo, error) {
	return repo.Repo{Name: name}, nil
}

func setup(t *testing.T) (*persistence.Persister, repo.Repo) {
	t.Helper()
	p := persistencetest.NewMemoryPersister(t)
	rp := persistence.NewRepoPersister(p)
	name, _ := repo.ParseFullName("MilesChou/devpulse")
	r, err := rp.EnsureID(context.Background(), "github", name)
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}
	return p, r
}

func TestFetch_PersistsBuildsAndPRs(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)

	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	loginAlice := "alice"

	ci := &fakeCIProvider{
		builds: []build.Build{
			{
				ExternalID: "9001",
				CommitSHA:  sha,
				Status:     build.StatusPassed,
				Trigger:    build.TriggerPush,
				Branch:     "main",
				StartedAt:  time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	ready := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	merged := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)
	vcs := &fakeVCSProvider{
		pulls: []pullrequest.PullRequest{
			{
				RepoID:    r.ID,
				Number:    42,
				Author:    "alice",
				Status:    pullrequest.StatusMerged,
				CreatedAt: created,
				ReadyAt:   &ready,
				MergedAt:  &merged,
			},
		},
		detail: pullrequest.PullRequest{Additions: 50, Deletions: 20},
		reviews: []pullrequest.Review{
			{ReviewerAccount: "bob", State: pullrequest.ReviewStateCommented,
				SubmittedAt: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
			{ReviewerAccount: "carol", State: pullrequest.ReviewStateApproved,
				SubmittedAt: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		},
		logins: map[commitsha.SHA]*string{sha: &loginAlice},
	}

	orch := fetching.NewOrchestrator(ci, vcs, bp, pp, rvp, nil)
	month := fetching.NewMonthRange(2026, time.May)

	buildOutcome := orch.FetchBuilds(ctx, r, month)
	if buildOutcome.Error != nil {
		t.Fatalf("build outcome err: %v", buildOutcome.Error)
	}
	if buildOutcome.BuildsWritten != 1 {
		t.Fatalf("builds written: %+v", buildOutcome)
	}

	prOutcome := orch.FetchPullRequests(ctx, r, month)
	if prOutcome.Error != nil {
		t.Fatalf("pr outcome err: %v", prOutcome.Error)
	}
	if prOutcome.PullRequestsWritten != 1 {
		t.Fatalf("prs written: %+v", prOutcome)
	}

	// Build author should have been back-filled.
	missing, _ := bp.ListMissingAuthorSHAs(ctx, r.ID, month)
	if len(missing) != 0 {
		t.Fatalf("expected no missing authors, got %v", missing)
	}

	// PR enrichment should be populated.
	got, err := pp.FindByNumber(ctx, r.ID, 42)
	if err != nil {
		t.Fatalf("find pr: %v", err)
	}
	if got.Additions != 50 || got.Deletions != 20 || got.TotalChangedLines != 70 {
		t.Fatalf("change stats wrong: %+v", got)
	}
	if got.FirstApprovedAt == nil || !got.FirstApprovedAt.Equal(vcs.reviews[1].SubmittedAt) {
		t.Fatalf("FirstApprovedAt: %v", got.FirstApprovedAt)
	}
	if got.TimeToApproval == nil || *got.TimeToApproval != 2*3600 {
		t.Fatalf("TimeToApproval: %v", got.TimeToApproval)
	}
	if got.TimeToMerge == nil || *got.TimeToMerge != 3*3600 {
		t.Fatalf("TimeToMerge: %v", got.TimeToMerge)
	}
}

func TestEnrichOnePullRequestByNumber_NotFound(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	orch := fetching.NewOrchestrator(&fakeCIProvider{}, &fakeVCSProvider{}, bp, pp, rvp, nil)
	_, err := orch.EnrichOnePullRequestByNumber(ctx, r, 999)
	if err == nil {
		t.Fatalf("expected error for missing PR")
	}
}

func TestFetch_BuildAuthorEnrichmentFailureDoesNotAbortPRFetch(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	ready := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

	ci := &fakeCIProvider{builds: []build.Build{
		{ExternalID: "1", CommitSHA: sha, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)},
	}}
	vcs := &fakeVCSProvider{
		pulls: []pullrequest.PullRequest{
			{RepoID: r.ID, Number: 1, Author: "x", Status: pullrequest.StatusOpen, CreatedAt: created, ReadyAt: &ready},
		},
		bulkErr: context.DeadlineExceeded, // simulate transient API failure
	}

	orch := fetching.NewOrchestrator(ci, vcs, bp, pp, rvp, nil)
	month := fetching.NewMonthRange(2026, time.May)

	// Author backfill failure inside FetchBuilds is swallowed (logged
	// only); the build itself is still written.
	buildOutcome := orch.FetchBuilds(ctx, r, month)
	if buildOutcome.Error != nil {
		t.Fatalf("FetchBuilds should not surface bulk-author failure: %v", buildOutcome.Error)
	}
	if buildOutcome.BuildsWritten != 1 {
		t.Fatalf("build not written: %+v", buildOutcome)
	}

	prOutcome := orch.FetchPullRequests(ctx, r, month)
	if prOutcome.Error != nil {
		t.Fatalf("FetchPullRequests should still succeed: %v", prOutcome.Error)
	}
	if prOutcome.PullRequestsWritten != 1 {
		t.Fatalf("PR not written: %+v", prOutcome)
	}
}
