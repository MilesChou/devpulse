package fetching_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

// fakeCIProvider returns canned builds and records the cursor it
// was called with. The recorder is mutex-guarded so future
// t.Parallel() tests do not race.
type fakeCIProvider struct {
	name   string // Name() falls back to "fake-ci" when empty
	builds []build.Build
	err    error

	mu       sync.Mutex
	calls    int
	gotSince time.Time
}

func (f *fakeCIProvider) Name() string {
	if f.name == "" {
		return "fake-ci"
	}
	return f.name
}

func (f *fakeCIProvider) ListBuildsSince(
	_ context.Context,
	_ repo.FullName,
	since time.Time,
) ([]build.Build, error) {
	f.mu.Lock()
	f.calls++
	f.gotSince = since
	f.mu.Unlock()
	return f.builds, f.err
}

func (f *fakeCIProvider) snapshot() (calls int, gotSince time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.gotSince
}

// fakeVCSProvider stubs out every method the orchestrator calls. The PR
// store is map-keyed by number so the by-number backfill flow can be
// driven precisely: omit a number to simulate 404 (issue / deleted),
// or set a `getErrFor` entry to simulate a transient failure on that
// specific PR.
type fakeVCSProvider struct {
	latestNumber int
	prs          map[int]pullrequest.PullRequest
	getErrFor    map[int]error
	reviews      []pullrequest.Review
	logins       map[commitsha.SHA]*string
	revsErr      error
	bulkErr      error
	latestErr    error

	// gotNumbers records every number GetPullRequest was called with, in
	// call order, so tests can assert the loop's traversal.
	gotNumbers []int
}

func (f *fakeVCSProvider) GetLatestPRNumber(
	_ context.Context, _ repo.FullName,
) (int, error) {
	return f.latestNumber, f.latestErr
}

func (f *fakeVCSProvider) GetPullRequest(
	_ context.Context, _ string, _ repo.FullName, number int,
) (pullrequest.PullRequest, error) {
	f.gotNumbers = append(f.gotNumbers, number)
	if err, ok := f.getErrFor[number]; ok {
		return pullrequest.PullRequest{}, err
	}
	pr, ok := f.prs[number]
	if !ok {
		return pullrequest.PullRequest{}, fmt.Errorf("%w: pr #%d", fetching.ErrNotFound, number)
	}
	return pr, nil
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

// makePR is a small helper to keep tests legible — only the fields the
// orchestrator actually inspects are set.
func makePR(repoID string, number int) pullrequest.PullRequest {
	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	ready := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	merged := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)
	return pullrequest.PullRequest{
		RepoID:    repoID,
		Number:    number,
		Author:    "alice",
		Status:    pullrequest.StatusMerged,
		Additions: 50,
		Deletions: 20,
		CreatedAt: created,
		ReadyAt:   &ready,
		MergedAt:  &merged,
	}
}

func TestBackfill_PersistsBuildsAndPRs(t *testing.T) {
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

	vcs := &fakeVCSProvider{
		latestNumber: 42,
		prs:          map[int]pullrequest.PullRequest{42: makePR(r.ID, 42)},
		reviews: []pullrequest.Review{
			{ReviewerAccount: "bob", State: pullrequest.ReviewStateCommented,
				SubmittedAt: time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)},
			{ReviewerAccount: "carol", State: pullrequest.ReviewStateApproved,
				SubmittedAt: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)},
		},
		logins: map[commitsha.SHA]*string{sha: &loginAlice},
	}

	orch := fetching.NewOrchestrator([]fetching.CIProvider{ci}, vcs, bp, pp, rvp, nil)

	buildsWritten, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds err: %v", err)
	}
	if buildsWritten != 1 {
		t.Fatalf("builds written: %d", buildsWritten)
	}

	prsWritten, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil {
		t.Fatalf("BackfillPullRequestsByNumber err: %v", err)
	}
	if prsWritten != 1 {
		t.Fatalf("prs written: %d", prsWritten)
	}

	// Author backfill: the build's commit had a NULL author before
	// enrichment; with logins[sha] = &alice the row should be populated.
	missing, err := bp.ListMissingAuthorSHAs(ctx, r.ID)
	if err != nil {
		t.Fatalf("list missing shas: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected zero missing-author SHAs, got %v", missing)
	}

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

// TestBackfill_StartsAtPRSyncStartNumber asserts that PRs below the
// configured floor are not even probed — this is how an operator skips
// early no-CI history.
func TestBackfill_StartsAtPRSyncStartNumber(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	r.PRSyncStartNumber = 5

	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	prs := map[int]pullrequest.PullRequest{}
	for n := 5; n <= 7; n++ {
		prs[n] = makePR(r.ID, n)
	}
	vcs := &fakeVCSProvider{latestNumber: 7, prs: prs}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if written != 3 {
		t.Fatalf("written: %d", written)
	}
	if got := vcs.gotNumbers; len(got) != 3 || got[0] != 5 || got[2] != 7 {
		t.Fatalf("expected probes 5..7, got %v", got)
	}
}

// TestBackfill_ResumesFromDBMax asserts derived-state resume: after a
// PR exists in the DB, the next backfill starts at MAX(number)+1 and
// does not re-probe earlier numbers.
func TestBackfill_ResumesFromDBMax(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	// Pre-seed PR #3 directly.
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{makePR(r.ID, 3)}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	prs := map[int]pullrequest.PullRequest{
		3: makePR(r.ID, 3),
		4: makePR(r.ID, 4),
		5: makePR(r.ID, 5),
	}
	vcs := &fakeVCSProvider{latestNumber: 5, prs: prs}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if written != 2 {
		t.Fatalf("written: %d (expected 2: #4, #5)", written)
	}
	if got := vcs.gotNumbers; len(got) != 2 || got[0] != 4 || got[1] != 5 {
		t.Fatalf("expected probes 4,5, got %v", got)
	}
}

// TestBackfill_404Skips asserts 404 doesn't abort the loop and isn't
// counted toward `written`.
func TestBackfill_404Skips(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	// #2 is missing (simulates an issue at that number).
	prs := map[int]pullrequest.PullRequest{
		1: makePR(r.ID, 1),
		3: makePR(r.ID, 3),
	}
	vcs := &fakeVCSProvider{latestNumber: 3, prs: prs}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if written != 2 {
		t.Fatalf("written: %d (expected 2; #2 was a 404)", written)
	}
	// All three numbers should have been probed.
	if got := vcs.gotNumbers; len(got) != 3 || got[1] != 2 {
		t.Fatalf("expected probes 1,2,3, got %v", got)
	}
}

// TestBackfill_FailsFastOnNonNotFoundError asserts that a real upstream
// failure on PR #N halts the loop immediately, leaving DB MAX at #N-1
// for derived-state resume on the next run.
func TestBackfill_FailsFastOnNonNotFoundError(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	prs := map[int]pullrequest.PullRequest{
		1: makePR(r.ID, 1),
		3: makePR(r.ID, 3),
	}
	boom := errors.New("simulated 500")
	vcs := &fakeVCSProvider{
		latestNumber: 3,
		prs:          prs,
		getErrFor:    map[int]error{2: boom},
	}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
	if written != 1 {
		t.Fatalf("written: %d (expected 1: #1 only, halted at #2)", written)
	}
	// Loop must stop at the failing number — must not probe #3 after #2 blew up.
	if got := vcs.gotNumbers; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected probes to stop at #2, got %v", got)
	}

	// DB MAX should be #1 — i.e. interrupted state recoverable.
	gotMax, has, err := pp.MaxNumber(ctx, r.ID)
	if err != nil {
		t.Fatalf("max number: %v", err)
	}
	if !has || gotMax != 1 {
		t.Fatalf("expected DB MAX = 1, got has=%v max=%d", has, gotMax)
	}
}

// TestBackfill_EmptyUpstream asserts an upstream with zero PRs returns
// (0, nil) — the loop short-circuits, no DB writes, no errors.
func TestBackfill_EmptyUpstream(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	vcs := &fakeVCSProvider{latestNumber: 0} // empty repo
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if written != 0 {
		t.Fatalf("written: %d (expected 0)", written)
	}
	if len(vcs.gotNumbers) != 0 {
		t.Fatalf("expected zero PR probes on empty repo, got %v", vcs.gotNumbers)
	}
}

// TestBackfill_StartNumberAboveRemoteMax asserts that an operator who
// sets PRSyncStartNumber above the upstream max gets a clean no-op
// (e.g. floor=1000 but only 200 PRs exist).
func TestBackfill_StartNumberAboveRemoteMax(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	r.PRSyncStartNumber = 1000

	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	vcs := &fakeVCSProvider{latestNumber: 200}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if written != 0 {
		t.Fatalf("written: %d (expected 0; start > remoteMax means nothing to do)", written)
	}
	if len(vcs.gotNumbers) != 0 {
		t.Fatalf("no PR should be probed, got %v", vcs.gotNumbers)
	}
}

// TestBackfill_ConfigVsDBPrecedence locks in the cursor formula
// start = max(PRSyncStartNumber, dbMax+1) for both orderings of the
// inputs.
func TestBackfill_ConfigVsDBPrecedence(t *testing.T) {
	mkPRs := func(repoID string, from, to int) map[int]pullrequest.PullRequest {
		m := map[int]pullrequest.PullRequest{}
		for n := from; n <= to; n++ {
			m[n] = makePR(repoID, n)
		}
		return m
	}

	t.Run("dbMax wins when higher than config", func(t *testing.T) {
		ctx := context.Background()
		p, r := setup(t)
		r.PRSyncStartNumber = 3
		bp := persistence.NewBuildPersister(p)
		pp := persistence.NewPullRequestPersister(p)
		rvp := persistence.NewReviewPersister(p)

		// Pre-seed up to #5 — dbMax=5 should override config floor of 3.
		seed := []pullrequest.PullRequest{makePR(r.ID, 5)}
		if _, err := pp.UpsertMany(ctx, seed); err != nil {
			t.Fatalf("seed: %v", err)
		}

		vcs := &fakeVCSProvider{latestNumber: 7, prs: mkPRs(r.ID, 5, 7)}
		orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)
		if _, err := orch.BackfillPullRequestsByNumber(ctx, r); err != nil {
			t.Fatalf("err: %v", err)
		}
		// Should probe 6,7 only — not 5 (already in DB) and not 3,4 (config floor not chosen).
		if got := vcs.gotNumbers; len(got) != 2 || got[0] != 6 || got[1] != 7 {
			t.Fatalf("expected [6 7], got %v", got)
		}
	})

	t.Run("config wins when higher than dbMax+1", func(t *testing.T) {
		ctx := context.Background()
		p, r := setup(t)
		r.PRSyncStartNumber = 10
		bp := persistence.NewBuildPersister(p)
		pp := persistence.NewPullRequestPersister(p)
		rvp := persistence.NewReviewPersister(p)

		seed := []pullrequest.PullRequest{makePR(r.ID, 5)}
		if _, err := pp.UpsertMany(ctx, seed); err != nil {
			t.Fatalf("seed: %v", err)
		}

		vcs := &fakeVCSProvider{latestNumber: 12, prs: mkPRs(r.ID, 10, 12)}
		orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)
		if _, err := orch.BackfillPullRequestsByNumber(ctx, r); err != nil {
			t.Fatalf("err: %v", err)
		}
		// Config floor 10 wins; dbMax+1=6 is too low. Probes 10,11,12.
		if got := vcs.gotNumbers; len(got) != 3 || got[0] != 10 || got[2] != 12 {
			t.Fatalf("expected [10 11 12], got %v", got)
		}
	})
}

// TestBackfill_GetLatestPRNumberError asserts that an upstream failure
// to read the max is surfaced and no DB writes happen.
func TestBackfill_GetLatestPRNumberError(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	boom := errors.New("simulated 502")
	vcs := &fakeVCSProvider{latestErr: boom}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
	if written != 0 {
		t.Fatalf("written: %d (expected 0)", written)
	}
}

// TestBackfill_CancelledCtxBreaksOut asserts that a cancelled ctx
// stops the orchestrator with a context.Canceled error. The exact
// stage at which the cancellation is observed (DB read of MaxNumber,
// or the loop's between-iteration ctx.Err() check) depends on call
// ordering — what matters is that no work happens after cancel.
func TestBackfill_CancelledCtxBreaksOut(t *testing.T) {
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	vcs := &fakeVCSProvider{
		latestNumber: 3,
		prs: map[int]pullrequest.PullRequest{
			1: makePR(r.ID, 1), 2: makePR(r.ID, 2), 3: makePR(r.ID, 3),
		},
	}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	written, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if written != 0 {
		t.Fatalf("written: %d (no PR should have been written)", written)
	}
}

// TestRepoPRSyncStartNumber_RoundTripsThroughDB asserts the new column
// survives a Create → FindByID round-trip with a non-default value.
func TestRepoPRSyncStartNumber_RoundTripsThroughDB(t *testing.T) {
	ctx := context.Background()
	persister := persistencetest.NewMemoryPersister(t)
	rp := persistence.NewRepoPersister(persister)

	name, _ := repo.ParseFullName("foo/bar")
	created, err := rp.Create(ctx, "github", name, "https://github.com/foo/bar",
		repo.Repo{PRSyncStartNumber: 1500})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.PRSyncStartNumber != 1500 {
		t.Fatalf("created.PRSyncStartNumber: %d", created.PRSyncStartNumber)
	}

	got, err := rp.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if got.PRSyncStartNumber != 1500 {
		t.Fatalf("round-trip lost PRSyncStartNumber: got %d", got.PRSyncStartNumber)
	}
}

// TestEnrichOnePullRequestByNumber_LocalDBMiss covers the case where
// `devpulse pr sync` is invoked on a PR that is not in the local store.
// Contract: (false, nil) — the CLI surface translates this to a
// "run `repo sync` first" hint. It is NOT an error condition at the
// orchestrator layer; treating it as such would prevent the CLI from
// rendering a useful message without depending on the persistence
// sentinel.
func TestEnrichOnePullRequestByNumber_LocalDBMiss(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, &fakeVCSProvider{}, bp, pp, rvp, nil)
	found, err := orch.EnrichOnePullRequestByNumber(ctx, r, 999)
	if err != nil {
		t.Fatalf("local miss should not error, got: %v", err)
	}
	if found {
		t.Fatalf("expected found=false when PR is not in the store")
	}
}

// TestEnrichOnePullRequestByNumber_UpstreamGone covers the case where
// the PR exists locally but has been deleted upstream. Unlike the
// backfill flow, this is reported as an error (operator-initiated
// command should not silently swallow a typo or upstream deletion).
func TestEnrichOnePullRequestByNumber_UpstreamGone(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	// Seed the PR locally.
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{makePR(r.ID, 42)}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// VCS doesn't know about #42 — returns ErrNotFound.
	vcs := &fakeVCSProvider{prs: map[int]pullrequest.PullRequest{}}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{&fakeCIProvider{}}, vcs, bp, pp, rvp, nil)

	found, err := orch.EnrichOnePullRequestByNumber(ctx, r, 42)
	if !found {
		t.Fatalf("expected found=true (PR exists locally)")
	}
	if err == nil {
		t.Fatalf("expected error when upstream returns 404")
	}
}

func TestFetch_BuildAuthorEnrichmentFailureDoesNotAbortPRFetch(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")

	ci := &fakeCIProvider{builds: []build.Build{
		{ExternalID: "1", CommitSHA: sha, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)},
	}}
	vcs := &fakeVCSProvider{
		latestNumber: 1,
		prs:          map[int]pullrequest.PullRequest{1: makePR(r.ID, 1)},
		bulkErr:      context.DeadlineExceeded, // simulate transient API failure
	}

	orch := fetching.NewOrchestrator([]fetching.CIProvider{ci}, vcs, bp, pp, rvp, nil)

	// Author backfill failure inside FetchAllBuilds is swallowed (logged
	// only); the build itself is still written.
	buildsWritten, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds should not surface bulk-author failure: %v", err)
	}
	if buildsWritten != 1 {
		t.Fatalf("build not written: %d", buildsWritten)
	}

	prsWritten, err := orch.BackfillPullRequestsByNumber(ctx, r)
	if err != nil {
		t.Fatalf("BackfillPullRequestsByNumber should succeed: %v", err)
	}
	if prsWritten != 1 {
		t.Fatalf("PR not written: %d", prsWritten)
	}
}

// TestFetchAllBuilds_ColdStart_PassesZeroSince asserts the first sync
// on an empty store calls the provider with a zero `since`, which the
// provider treats as cold-start (walk the full upstream history). This
// is the no-watermark branch of the incremental logic.
func TestFetchAllBuilds_ColdStart_PassesZeroSince(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	ci := &fakeCIProvider{builds: []build.Build{
		{ExternalID: "1", CommitSHA: sha, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)},
	}}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{ci}, &fakeVCSProvider{}, bp, pp, rvp, nil)

	written, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds: %v", err)
	}
	if written != 1 {
		t.Fatalf("written: %d (expected 1)", written)
	}
	calls, gotSince := ci.snapshot()
	if calls != 1 {
		t.Fatalf("provider calls: %d (expected 1)", calls)
	}
	if !gotSince.IsZero() {
		t.Fatalf("cold start should pass zero since, got %v", gotSince)
	}
}

// TestFetchAllBuilds_Incremental_PassesWatermarkMinusOverlap asserts
// the second-and-onward sync derives the watermark from the provider's
// own MAX(started_at) and subtracts the overlap window before handing
// it to the provider. This locks in the contract between the
// orchestrator and the persister so a future change to the overlap
// constant has to land in lock-step with this assertion.
func TestFetchAllBuilds_Incremental_PassesWatermarkMinusOverlap(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	shaSeed, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	seedStarted := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	if _, err := bp.UpsertMany(ctx, r.ID, "fake-ci", []build.Build{
		{ExternalID: "100", CommitSHA: shaSeed, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: seedStarted},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Provider returns one fresh build past the watermark.
	shaNew, _ := commitsha.Parse("bbb1234567890abcdef1234567890abcdef12345")
	ci := &fakeCIProvider{builds: []build.Build{
		{ExternalID: "200", CommitSHA: shaNew, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: seedStarted.Add(2 * time.Hour)},
	}}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{ci}, &fakeVCSProvider{}, bp, pp, rvp, nil)

	written, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds: %v", err)
	}
	if written != 1 {
		t.Fatalf("written: %d (expected 1)", written)
	}
	wantSince := seedStarted.Add(-6 * time.Hour) // watermark - buildOverlap
	_, gotSince := ci.snapshot()
	if !gotSince.Equal(wantSince) {
		t.Fatalf("since: got %v, want %v (watermark - 6h overlap)", gotSince, wantSince)
	}
}

// TestFetchAllBuilds_PerProviderWatermark asserts that each provider
// gets its own cursor: rows already stored for one provider must not
// advance another provider past its cold start. This is the regression
// guard for the multi-provider data-loss scenario — enabling a second
// CI provider on a store that already has history from the first one
// must walk the new provider's full history.
func TestFetchAllBuilds_PerProviderWatermark(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	shaSeed, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	seedStarted := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	if _, err := bp.UpsertMany(ctx, r.ID, "travis", []build.Build{
		{ExternalID: "100", CommitSHA: shaSeed, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: seedStarted},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	travisCI := &fakeCIProvider{name: "travis"}
	actionsCI := &fakeCIProvider{name: "github-actions"}
	orch := fetching.NewOrchestrator(
		[]fetching.CIProvider{travisCI, actionsCI}, &fakeVCSProvider{}, bp, pp, rvp, nil)

	if _, err := orch.FetchAllBuilds(ctx, r); err != nil {
		t.Fatalf("FetchAllBuilds: %v", err)
	}

	_, travisSince := travisCI.snapshot()
	wantTravis := seedStarted.Add(-6 * time.Hour) // its own watermark - overlap
	if !travisSince.Equal(wantTravis) {
		t.Fatalf("travis since: got %v, want %v", travisSince, wantTravis)
	}

	_, actionsSince := actionsCI.snapshot()
	if !actionsSince.IsZero() {
		t.Fatalf("github-actions has no rows; expected zero since (cold start), got %v", actionsSince)
	}
}

// TestFetchAllBuilds_SameExternalIDAcrossProviders asserts that two
// providers may emit the same external ID without colliding — the
// dedupe key is (repo_id, ci_provider, external_id), not
// (repo_id, external_id). Travis build IDs and Actions run IDs are
// both plain integers, so cross-provider collisions are plausible.
func TestFetchAllBuilds_SameExternalIDAcrossProviders(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	started := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	mk := func(name string) *fakeCIProvider {
		return &fakeCIProvider{name: name, builds: []build.Build{
			{ExternalID: "42", CommitSHA: sha, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
				StartedAt: started},
		}}
	}

	orch := fetching.NewOrchestrator(
		[]fetching.CIProvider{mk("travis"), mk("github-actions")},
		&fakeVCSProvider{}, bp, pp, rvp, nil)

	written, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds: %v", err)
	}
	if written != 2 {
		t.Fatalf("written: %d (expected 2 — same external ID, different providers)", written)
	}
}

// TestFetchAllBuilds_RetryWithinOverlap_DedupedByDB exercises the
// safety net that the overlap window buys us: when the
// provider returns a build whose external_id was already stored
// (e.g. the same row resurfaces because the page boundary fell inside
// the overlap), UpsertMany silently dedupes it via the
// (repo_id, ci_provider, external_id) unique constraint, while a genuinely new
// retry build with a fresh external_id is still written even though
// its started_at lands before the watermark. The net `written` count
// is the count of truly new rows.
func TestFetchAllBuilds_RetryWithinOverlap_DedupedByDB(t *testing.T) {
	ctx := context.Background()
	p, r := setup(t)
	bp := persistence.NewBuildPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)

	shaSeed, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	watermark := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	if _, err := bp.UpsertMany(ctx, r.ID, "fake-ci", []build.Build{
		{ExternalID: "100", CommitSHA: shaSeed, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: watermark},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Provider replays: ID 100 (already in DB; resurfaces because the
	// overlap window made the page include it) and ID 101 (a retry
	// build that started 2 minutes before the watermark — within the 	// overlap window, so it survives the page-level stop).
	shaRetry, _ := commitsha.Parse("bbb1234567890abcdef1234567890abcdef12345")
	ci := &fakeCIProvider{builds: []build.Build{
		{ExternalID: "100", CommitSHA: shaSeed, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: watermark},
		{ExternalID: "101", CommitSHA: shaRetry, Status: build.StatusPassed, Trigger: build.TriggerPush, Branch: "main",
			StartedAt: watermark.Add(-2 * time.Minute)},
	}}
	orch := fetching.NewOrchestrator([]fetching.CIProvider{ci}, &fakeVCSProvider{}, bp, pp, rvp, nil)

	written, err := orch.FetchAllBuilds(ctx, r)
	if err != nil {
		t.Fatalf("FetchAllBuilds: %v", err)
	}
	if written != 1 {
		t.Fatalf("written: %d (expected 1 — the retry build only; ID 100 is a no-op via ON CONFLICT)", written)
	}

	// The watermark should NOT have moved backward — even though we
	// upserted a row with started_at before it, MAX(started_at) stays
	// at the original watermark (the retry started earlier than the
	// previous max).
	maxStarted, has, err := bp.MaxStartedAt(ctx, r.ID, "fake-ci")
	if err != nil {
		t.Fatalf("MaxStartedAt: %v", err)
	}
	if !has || !maxStarted.Equal(watermark) {
		t.Fatalf("watermark after retry: has=%v got=%v want=%v", has, maxStarted, watermark)
	}
}
