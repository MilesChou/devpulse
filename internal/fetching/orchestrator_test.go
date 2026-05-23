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

func (f *fakeCIProvider) ListAllBuilds(
	_ context.Context,
	_ repo.FullName,
) ([]build.Build, error) {
	return f.builds, f.err
}

// fakeVCSProvider stubs out every method the orchestrator calls.
type fakeVCSProvider struct {
	allPulls []pullrequest.PullRequest
	detail   pullrequest.PullRequest
	reviews  []pullrequest.Review
	logins   map[commitsha.SHA]*string
	getErr   error
	revsErr  error
	bulkErr  error
}

func (f *fakeVCSProvider) ListAllPullRequestsPageFunc(
	_ context.Context, _ string, _ repo.FullName,
	fn func(page []pullrequest.PullRequest) error,
) error {
	if len(f.allPulls) == 0 {
		return nil
	}
	return fn(f.allPulls)
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
		allPulls: []pullrequest.PullRequest{
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

	buildsWritten, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds err: %v", err)
	}
	if buildsWritten != 1 {
		t.Fatalf("builds written: %d", buildsWritten)
	}

	prsWritten, err := orch.FetchAllPullRequestsWithEnrichment(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllPullRequestsWithEnrichment err: %v", err)
	}
	if prsWritten != 1 {
		t.Fatalf("prs written: %d", prsWritten)
	}

	// Author backfill: the build's commit had a NULL author before
	// enrichment; with logins[sha] = &alice the row should be populated
	// and ListMissingAuthorSHAs should return nothing for the repo.
	missing, err := bp.ListMissingAuthorSHAs(ctx, r.ID)
	if err != nil {
		t.Fatalf("list missing shas: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected zero missing-author SHAs after enrichment, got %v", missing)
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
		allPulls: []pullrequest.PullRequest{
			{RepoID: r.ID, Number: 1, Author: "x", Status: pullrequest.StatusOpen, CreatedAt: created, ReadyAt: &ready},
		},
		bulkErr: context.DeadlineExceeded, // simulate transient API failure
	}

	orch := fetching.NewOrchestrator(ci, vcs, bp, pp, rvp, nil)

	// Author backfill failure inside FetchAllBuilds is swallowed (logged
	// only); the build itself is still written.
	buildsWritten, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds should not surface bulk-author failure: %v", err)
	}
	if buildsWritten != 1 {
		t.Fatalf("build not written: %d", buildsWritten)
	}

	prsWritten, err := orch.FetchAllPullRequestsWithEnrichment(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllPullRequestsWithEnrichment should still succeed: %v", err)
	}
	if prsWritten != 1 {
		t.Fatalf("PR not written: %d", prsWritten)
	}
}
