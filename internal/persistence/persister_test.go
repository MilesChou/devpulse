package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/persistencetest"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

func setup(t *testing.T) *persistence.Persister {
	return persistencetest.NewMemoryPersister(t)
}

func mustFullName(t *testing.T, s string) repo.FullName {
	t.Helper()
	n, err := repo.ParseFullName(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return n
}

func TestRepoPersister_UpdatePRSyncStart(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ctx := context.Background()

	r, err := rp.EnsureID(ctx, "github", mustFullName(t, "foo/bar"))
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if r.PRSyncStartNumber != 1 {
		t.Fatalf("default should be 1, got %d", r.PRSyncStartNumber)
	}

	if err := rp.UpdatePRSyncStart(ctx, r.ID, 500); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := rp.FindByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.PRSyncStartNumber != 500 {
		t.Fatalf("expected 500, got %d", got.PRSyncStartNumber)
	}
}

func TestRepoPersister_UpdatePRSyncStart_RejectsBelowOne(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "foo/bar"))

	// 0 and negatives are rejected at the Go layer (defense-in-depth
	// alongside the DB CHECK constraint).
	for _, n := range []int{0, -1, -1000} {
		if err := rp.UpdatePRSyncStart(ctx, r.ID, n); err == nil {
			t.Fatalf("expected error for n=%d", n)
		}
	}
}

func TestRepoPersister_UpdatePRSyncStart_RepoNotFound(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ctx := context.Background()

	err := rp.UpdatePRSyncStart(ctx, "01HZZZZZZZZZZZZZZZZZZZZZZZ", 5)
	if err == nil {
		t.Fatalf("expected error for missing repo")
	}
	if !errors.Is(err, persistence.ErrRepoNotFound) {
		t.Fatalf("expected ErrRepoNotFound, got %v", err)
	}
}

func TestRepoPersister_EnsureID_CreatesThenReuses(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ctx := context.Background()

	r1, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure 1: %v", err)
	}
	if r1.ID == "" {
		t.Fatalf("id empty")
	}
	if r1.Name.String() != "MilesChou/devpulse" {
		t.Fatalf("name mismatch")
	}

	r2, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure 2: %v", err)
	}
	if r2.ID != r1.ID {
		t.Fatalf("second ensure should reuse id; got %q vs %q", r2.ID, r1.ID)
	}
}

func TestRepoPersister_ListAll(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	ctx := context.Background()

	// Empty store: must return an empty (non-nil) slice, not an error.
	got, err := rp.ListAll(ctx)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if got == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(got))
	}

	// Insert two; mark one disabled. EnsureID does not set Disabled, so
	// update metadata to flip it.
	r1, err := rp.EnsureID(ctx, "github", mustFullName(t, "zeta/one"))
	if err != nil {
		t.Fatalf("ensure r1: %v", err)
	}
	r2, err := rp.EnsureID(ctx, "github", mustFullName(t, "alpha/two"))
	if err != nil {
		t.Fatalf("ensure r2: %v", err)
	}
	if err := rp.UpdateMetadata(ctx, r2.ID, repo.Repo{
		DefaultBranch: "main",
		Disabled:      true,
	}); err != nil {
		t.Fatalf("flip disabled: %v", err)
	}

	got, err = rp.ListAll(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(got))
	}

	// ORDER BY owner, repo_name: alpha/two should come before zeta/one.
	if got[0].Name.String() != "alpha/two" {
		t.Fatalf("expected alpha/two first, got %q", got[0].Name.String())
	}
	if !got[0].Disabled {
		t.Fatalf("expected alpha/two disabled=true")
	}
	if got[1].Name.String() != "zeta/one" {
		t.Fatalf("expected zeta/one second, got %q", got[1].Name.String())
	}
	if got[1].Disabled {
		t.Fatalf("expected zeta/one disabled=false")
	}
	// IDs should round-trip.
	if got[0].ID != r2.ID || got[1].ID != r1.ID {
		t.Fatalf("id mismatch: got [%q, %q], want [%q, %q]",
			got[0].ID, got[1].ID, r2.ID, r1.ID)
	}
}

func TestBuildPersister_UpsertMany_Idempotent(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	bp := persistence.NewBuildPersister(p)
	ctx := context.Background()

	r, err := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	if err != nil {
		t.Fatalf("ensure repo: %v", err)
	}

	sha, _ := commitsha.Parse("abc1234567890abcdef1234567890abcdef12345")
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	builds := []build.Build{
		{
			ExternalID: "travis-1",
			RepoID:     r.ID,
			CommitSHA:  sha,
			Branch:     "main",
			Status:     build.StatusPassed,
			Trigger:    build.TriggerPush,
			StartedAt:  now,
		},
		{
			ExternalID: "travis-2",
			RepoID:     r.ID,
			CommitSHA:  sha,
			Branch:     "main",
			Status:     build.StatusFailed,
			Trigger:    build.TriggerPullRequest,
			StartedAt:  now.Add(time.Hour),
		},
	}

	written, err := bp.UpsertMany(ctx, r.ID, builds)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if written != 2 {
		t.Fatalf("expected 2 written, got %d", written)
	}

	// Same payload again must not insert duplicates.
	written2, err := bp.UpsertMany(ctx, r.ID, builds)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if written2 != 0 {
		t.Fatalf("expected 0 written on duplicate, got %d", written2)
	}
}

func TestBuildPersister_ListMissingAuthorSHAs(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	bp := persistence.NewBuildPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))

	shaA, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	shaB, _ := commitsha.Parse("bbb1234567890abcdef1234567890abcdef12345")

	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	_, err := bp.UpsertMany(ctx, r.ID, []build.Build{
		{ExternalID: "1", RepoID: r.ID, CommitSHA: shaA, Status: build.StatusPassed, StartedAt: now},
		{ExternalID: "2", RepoID: r.ID, CommitSHA: shaB, Status: build.StatusPassed, StartedAt: now, Author: "alice"},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	missing, err := bp.ListMissingAuthorSHAs(ctx, r.ID)
	if err != nil {
		t.Fatalf("list missing: %v", err)
	}
	if len(missing) != 1 || missing[0] != shaA {
		t.Fatalf("expected [%s], got %v", shaA, missing)
	}
}

func TestBuildPersister_UpdateAuthorBySHA(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	bp := persistence.NewBuildPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	_, _ = bp.UpsertMany(ctx, r.ID, []build.Build{
		{ExternalID: "1", RepoID: r.ID, CommitSHA: sha, Status: build.StatusPassed, StartedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
	})

	if err := bp.UpdateAuthorBySHA(ctx, r.ID, sha, "alice"); err != nil {
		t.Fatalf("update: %v", err)
	}

	missing, _ := bp.ListMissingAuthorSHAs(ctx, r.ID)
	if len(missing) != 0 {
		t.Fatalf("expected no missing after update, got %v", missing)
	}
}

// TestBuildPersister_MaxStartedAt round-trips the watermark query for
// both branches: empty store → (zero, false, nil) is the cold-start
// signal the orchestrator depends on; populated store → MAX(started_at)
// over the rows, in UTC. The driver-type switch in MaxStartedAt is
// exercised end-to-end here because the seeded rows are written via
// UpsertMany using the real driver path.
func TestBuildPersister_MaxStartedAt(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	bp := persistence.NewBuildPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))

	// Empty store: cold-start signal.
	zero, has, err := bp.MaxStartedAt(ctx, r.ID)
	if err != nil {
		t.Fatalf("max on empty: %v", err)
	}
	if has {
		t.Fatalf("expected has=false on empty store, got max=%v", zero)
	}

	sha, _ := commitsha.Parse("aaa1234567890abcdef1234567890abcdef12345")
	earlier := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	later := time.Date(2026, 5, 1, 18, 0, 0, 0, time.UTC)

	_, err = bp.UpsertMany(ctx, r.ID, []build.Build{
		{ExternalID: "1", RepoID: r.ID, CommitSHA: sha, Status: build.StatusPassed, StartedAt: earlier},
		{ExternalID: "2", RepoID: r.ID, CommitSHA: sha, Status: build.StatusPassed, StartedAt: later},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, has, err := bp.MaxStartedAt(ctx, r.ID)
	if err != nil {
		t.Fatalf("max: %v", err)
	}
	if !has {
		t.Fatalf("expected has=true after seed")
	}
	if !got.Equal(later) {
		t.Fatalf("max: got %v, want %v", got, later)
	}
}

// TestPullRequestPersister_Upsert_RoundTripsAllFields asserts every
// column the sync flow writes (basic + change stats + enrichment) is
// readable via FindByNumber. With the by-number sync, UpsertMany is the
// single write path — there is no separate UpdateEnrichment step.
func TestPullRequestPersister_Upsert_RoundTripsAllFields(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))

	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	ready := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	firstReview := time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)
	approve := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	merged := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)
	timeToApproval := 2 * 3600
	timeToMerge := 3 * 3600

	full := pullrequest.PullRequest{
		RepoID:            r.ID,
		Number:            42,
		Author:            "alice",
		Status:            pullrequest.StatusMerged,
		Additions:         50,
		Deletions:         20,
		TotalChangedLines: 70,
		IsDraft:           false,
		CreatedAt:         created,
		ReadyAt:           &ready,
		FirstReviewAt:     &firstReview,
		FirstApprovedAt:   &approve,
		TimeToApproval:    &timeToApproval,
		TimeToMerge:       &timeToMerge,
		MergedAt:          &merged,
	}
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{full}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := pp.FindByNumber(ctx, r.ID, 42)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Author != "alice" || got.Status != pullrequest.StatusMerged {
		t.Fatalf("basic fields: %+v", got)
	}
	if got.Additions != 50 || got.Deletions != 20 || got.TotalChangedLines != 70 {
		t.Fatalf("change stats: %+v", got)
	}
	if got.FirstReviewAt == nil || !got.FirstReviewAt.Equal(firstReview) {
		t.Fatalf("first_review_at: %v", got.FirstReviewAt)
	}
	if got.FirstApprovedAt == nil || !got.FirstApprovedAt.Equal(approve) {
		t.Fatalf("first_approved_at: %v", got.FirstApprovedAt)
	}
	if got.TimeToApproval == nil || *got.TimeToApproval != timeToApproval {
		t.Fatalf("time_to_approval: %v", got.TimeToApproval)
	}
	if got.TimeToMerge == nil || *got.TimeToMerge != timeToMerge {
		t.Fatalf("time_to_merge: %v", got.TimeToMerge)
	}
}

// TestPullRequestPersister_Upsert_OverwritesEnrichmentOnConflict asserts
// the new contract: re-upserting the same PR number with fresh values
// overwrites every mutable column, including additions/deletions and
// lead-time fields. This is what lets `devpulse pr sync <n>` repair a
// row whose backfill was interrupted mid-PR.
func TestPullRequestPersister_Upsert_OverwritesEnrichmentOnConflict(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	// First write: enrichment is missing (zero/nil), Status=Open.
	first := pullrequest.PullRequest{
		RepoID: r.ID, Number: 42, Author: "alice",
		Status: pullrequest.StatusOpen, CreatedAt: created,
	}
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{first}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Second write: Status=Merged + enrichment present. Author is
	// immutable (DO UPDATE excludes it) but everything else should refresh.
	approve := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	approval := 3600
	second := pullrequest.PullRequest{
		RepoID: r.ID, Number: 42, Author: "alice",
		Status:            pullrequest.StatusMerged,
		Additions:         10,
		Deletions:         5,
		TotalChangedLines: 15,
		CreatedAt:         created,
		FirstApprovedAt:   &approve,
		TimeToApproval:    &approval,
	}
	if _, err := pp.UpsertMany(ctx, []pullrequest.PullRequest{second}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, _ := pp.FindByNumber(ctx, r.ID, 42)
	if got.Status != pullrequest.StatusMerged {
		t.Fatalf("status not refreshed: %v", got.Status)
	}
	if got.Additions != 10 || got.Deletions != 5 {
		t.Fatalf("change stats not refreshed: a=%d d=%d", got.Additions, got.Deletions)
	}
	if got.TimeToApproval == nil || *got.TimeToApproval != approval {
		t.Fatalf("time_to_approval not refreshed: %v", got.TimeToApproval)
	}
	if got.FirstApprovedAt == nil || !got.FirstApprovedAt.Equal(approve) {
		t.Fatalf("first_approved_at not refreshed: %v", got.FirstApprovedAt)
	}
}

func TestReviewPersister_Upsert(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	rvp := persistence.NewReviewPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	_, _ = pp.UpsertMany(ctx, []pullrequest.PullRequest{
		{RepoID: r.ID, Number: 42, Author: "alice", Status: pullrequest.StatusOpen, CreatedAt: time.Now().UTC()},
	})
	pr, _ := pp.FindByNumber(ctx, r.ID, 42)

	t1 := time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)
	if err := rvp.Upsert(ctx, pr.ID, pullrequest.Review{
		ReviewerAccount: "bob", State: pullrequest.ReviewStateCommented, SubmittedAt: t1,
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Promote the same (pr, reviewer, time) review from commented to approved.
	if err := rvp.Upsert(ctx, pr.ID, pullrequest.Review{
		ReviewerAccount: "bob", State: pullrequest.ReviewStateApproved, SubmittedAt: t1,
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// Verify there's exactly one row, with state=approved.
	var count int
	var state string
	q := p.Rebind(`SELECT COUNT(*), MAX(state) FROM pull_request_reviews WHERE pull_request_id = ?`)
	if err := p.DB.QueryRowContext(ctx, q, pr.ID).Scan(&count, &state); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
	if state != "approved" {
		t.Fatalf("expected state=approved, got %q", state)
	}
}
