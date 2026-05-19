package persistence_test

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

	month := fetching.NewMonthRange(2026, time.May)
	missing, err := bp.ListMissingAuthorSHAs(ctx, r.ID, month)
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

	month := fetching.NewMonthRange(2026, time.May)
	missing, _ := bp.ListMissingAuthorSHAs(ctx, r.ID, month)
	if len(missing) != 0 {
		t.Fatalf("expected no missing after update, got %v", missing)
	}
}

func TestPullRequestPersister_UpsertThenEnrichment(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))

	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	ready := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	merged := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)

	prs := []pullrequest.PullRequest{
		{
			RepoID:    r.ID,
			Number:    42,
			Author:    "alice",
			Status:    pullrequest.StatusMerged,
			IsDraft:   false,
			CreatedAt: created,
			ReadyAt:   &ready,
			MergedAt:  &merged,
		},
	}
	if _, err := pp.UpsertMany(ctx, prs); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := pp.FindByNumber(ctx, r.ID, 42)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Author != "alice" {
		t.Fatalf("author %q", got.Author)
	}
	if got.Status != pullrequest.StatusMerged {
		t.Fatalf("status %v", got.Status)
	}

	approve := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	firstReview := time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC)
	timeToApproval := 2 * 3600
	timeToMerge := 3 * 3600

	if err := pp.UpdateEnrichment(ctx, got.ID, pullrequest.EnrichmentPatch{
		Additions:         50,
		Deletions:         20,
		TotalChangedLines: 70,
		FirstReviewAt:     &firstReview,
		FirstApprovedAt:   &approve,
		TimeToApproval:    &timeToApproval,
		TimeToMerge:       &timeToMerge,
	}); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	got2, _ := pp.FindByNumber(ctx, r.ID, 42)
	if got2.Additions != 50 || got2.Deletions != 20 || got2.TotalChangedLines != 70 {
		t.Fatalf("change stats: %+v", got2)
	}
	if got2.FirstReviewAt == nil || !got2.FirstReviewAt.Equal(firstReview) {
		t.Fatalf("first_review_at not stored: %v", got2.FirstReviewAt)
	}
	if got2.FirstApprovedAt == nil || !got2.FirstApprovedAt.Equal(approve) {
		t.Fatalf("first_approved_at not stored: %v", got2.FirstApprovedAt)
	}
	if got2.TimeToApproval == nil || *got2.TimeToApproval != timeToApproval {
		t.Fatalf("time_to_approval: %v", got2.TimeToApproval)
	}
	if got2.TimeToMerge == nil || *got2.TimeToMerge != timeToMerge {
		t.Fatalf("time_to_merge: %v", got2.TimeToMerge)
	}
}

func TestPullRequestPersister_Upsert_DoesNotClobberEnrichment(t *testing.T) {
	p := setup(t)
	rp := persistence.NewRepoPersister(p)
	pp := persistence.NewPullRequestPersister(p)
	ctx := context.Background()

	r, _ := rp.EnsureID(ctx, "github", mustFullName(t, "MilesChou/devpulse"))
	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)

	_, _ = pp.UpsertMany(ctx, []pullrequest.PullRequest{
		{RepoID: r.ID, Number: 42, Author: "alice", Status: pullrequest.StatusOpen, CreatedAt: created},
	})
	pr, _ := pp.FindByNumber(ctx, r.ID, 42)

	// Apply enrichment.
	a := 3600
	approve := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	_ = pp.UpdateEnrichment(ctx, pr.ID, pullrequest.EnrichmentPatch{
		Additions: 10, Deletions: 5, TotalChangedLines: 15,
		FirstApprovedAt: &approve, TimeToApproval: &a,
	})

	// Re-upsert the PR (e.g. month re-fetch). Status should refresh but
	// enrichment columns must remain.
	_, _ = pp.UpsertMany(ctx, []pullrequest.PullRequest{
		{ID: pr.ID, RepoID: r.ID, Number: 42, Author: "alice", Status: pullrequest.StatusMerged, CreatedAt: created},
	})

	got, _ := pp.FindByNumber(ctx, r.ID, 42)
	if got.Status != pullrequest.StatusMerged {
		t.Fatalf("status not refreshed: %v", got.Status)
	}
	if got.TimeToApproval == nil || *got.TimeToApproval != a {
		t.Fatalf("enrichment lost: %v", got.TimeToApproval)
	}
	if got.Additions != 10 {
		t.Fatalf("additions lost: %d", got.Additions)
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
