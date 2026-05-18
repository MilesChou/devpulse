package pullrequest

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func tp(t time.Time) *time.Time { return &t }
func ip(v int) *int             { return &v }

func TestChangeStats_Total(t *testing.T) {
	c := ChangeStats{Additions: 30, Deletions: 12}
	if got := c.Total(); got != 42 {
		t.Fatalf("got %d want 42", got)
	}
}

func TestAggregateReviews_IgnoresPreReadyReviews(t *testing.T) {
	readyAt := mustTime(t, "2026-05-01T10:00:00Z")
	preReady := mustTime(t, "2026-05-01T09:00:00Z")
	postReady1 := mustTime(t, "2026-05-01T11:00:00Z")
	postReady2 := mustTime(t, "2026-05-01T13:00:00Z")

	reviews := []Review{
		{ReviewerAccount: "alice", State: ReviewStateApproved, SubmittedAt: preReady},   // dropped
		{ReviewerAccount: "bob", State: ReviewStateCommented, SubmittedAt: postReady1},
		{ReviewerAccount: "carol", State: ReviewStateApproved, SubmittedAt: postReady2},
	}

	agg := AggregateReviews(reviews, &readyAt)

	if agg.FirstReviewAt == nil || !agg.FirstReviewAt.Equal(postReady1) {
		t.Fatalf("FirstReviewAt got %v want %v", agg.FirstReviewAt, postReady1)
	}
	if agg.FirstApprovedAt == nil || !agg.FirstApprovedAt.Equal(postReady2) {
		t.Fatalf("FirstApprovedAt got %v want %v", agg.FirstApprovedAt, postReady2)
	}
}

func TestAggregateReviews_NilReadyAt_IncludesAll(t *testing.T) {
	t1 := mustTime(t, "2026-05-01T08:00:00Z")
	t2 := mustTime(t, "2026-05-01T09:00:00Z")

	reviews := []Review{
		{State: ReviewStateApproved, SubmittedAt: t2},
		{State: ReviewStateCommented, SubmittedAt: t1},
	}

	agg := AggregateReviews(reviews, nil)

	if !agg.FirstReviewAt.Equal(t1) {
		t.Fatalf("FirstReviewAt got %v want %v", agg.FirstReviewAt, t1)
	}
	if !agg.FirstApprovedAt.Equal(t2) {
		t.Fatalf("FirstApprovedAt got %v want %v", agg.FirstApprovedAt, t2)
	}
}

func TestAggregateReviews_NoApproved_FirstApprovedNil(t *testing.T) {
	t1 := mustTime(t, "2026-05-01T10:00:00Z")
	reviews := []Review{
		{State: ReviewStateCommented, SubmittedAt: t1},
		{State: ReviewStateChangesRequested, SubmittedAt: t1.Add(time.Hour)},
	}
	agg := AggregateReviews(reviews, nil)
	if agg.FirstReviewAt == nil {
		t.Fatalf("expected FirstReviewAt set")
	}
	if agg.FirstApprovedAt != nil {
		t.Fatalf("expected FirstApprovedAt nil, got %v", *agg.FirstApprovedAt)
	}
}

func TestComputeTimeToApproval(t *testing.T) {
	ready := mustTime(t, "2026-05-01T10:00:00Z")
	approve := mustTime(t, "2026-05-01T12:00:00Z")

	if got := ComputeTimeToApproval(nil, &approve); got != nil {
		t.Fatalf("expected nil when readyAt missing, got %v", *got)
	}
	if got := ComputeTimeToApproval(&ready, nil); got != nil {
		t.Fatalf("expected nil when approvedAt missing, got %v", *got)
	}

	got := ComputeTimeToApproval(&ready, &approve)
	if got == nil || *got != 2*3600 {
		t.Fatalf("got %v want 7200", got)
	}
}

func TestComputeTimeToApproval_NegativeClampedToZero(t *testing.T) {
	ready := mustTime(t, "2026-05-01T12:00:00Z")
	approve := mustTime(t, "2026-05-01T11:00:00Z") // before ready, data anomaly
	got := ComputeTimeToApproval(&ready, &approve)
	if got == nil || *got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestComputeTimeToMerge(t *testing.T) {
	approve := mustTime(t, "2026-05-01T12:00:00Z")
	merge := mustTime(t, "2026-05-01T15:00:00Z")

	if got := ComputeTimeToMerge(nil, &merge); got != nil {
		t.Fatalf("expected nil when approvedAt missing, got %v", *got)
	}
	if got := ComputeTimeToMerge(&approve, nil); got != nil {
		t.Fatalf("expected nil when mergedAt missing, got %v", *got)
	}

	got := ComputeTimeToMerge(&approve, &merge)
	if got == nil || *got != 3*3600 {
		t.Fatalf("got %v want 10800", got)
	}
}

func TestComputeTimeToMerge_NegativeClampedToZero(t *testing.T) {
	approve := mustTime(t, "2026-05-01T15:00:00Z")
	merge := mustTime(t, "2026-05-01T12:00:00Z") // before approval, data anomaly
	got := ComputeTimeToMerge(&approve, &merge)
	if got == nil || *got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestBuildEnrichmentPatch_FillsEveryField(t *testing.T) {
	ready := mustTime(t, "2026-05-01T10:00:00Z")
	merged := mustTime(t, "2026-05-01T15:00:00Z")
	pr := PullRequest{
		ID:       "pr-1",
		Number:   42,
		ReadyAt:  &ready,
		MergedAt: &merged,
	}

	approve := mustTime(t, "2026-05-01T12:00:00Z")
	firstReview := mustTime(t, "2026-05-01T11:00:00Z")
	agg := ReviewAggregate{FirstReviewAt: &firstReview, FirstApprovedAt: &approve}

	patch := BuildEnrichmentPatch(pr, 50, 20, agg)

	if patch.Additions != 50 || patch.Deletions != 20 || patch.TotalChangedLines != 70 {
		t.Fatalf("change stats wrong: %+v", patch)
	}
	if !patch.FirstReviewAt.Equal(firstReview) {
		t.Fatalf("FirstReviewAt mismatch")
	}
	if !patch.FirstApprovedAt.Equal(approve) {
		t.Fatalf("FirstApprovedAt mismatch")
	}
	if patch.TimeToApproval == nil || *patch.TimeToApproval != 2*3600 {
		t.Fatalf("TimeToApproval got %v want 7200", patch.TimeToApproval)
	}
	if patch.TimeToMerge == nil || *patch.TimeToMerge != 3*3600 {
		t.Fatalf("TimeToMerge got %v want 10800", patch.TimeToMerge)
	}
}

func TestBuildEnrichmentPatch_NoReadyAt_TimeToApprovalNil(t *testing.T) {
	merged := mustTime(t, "2026-05-01T15:00:00Z")
	pr := PullRequest{ID: "pr-1", MergedAt: &merged}

	approve := mustTime(t, "2026-05-01T12:00:00Z")
	agg := ReviewAggregate{FirstApprovedAt: &approve}

	patch := BuildEnrichmentPatch(pr, 0, 0, agg)
	if patch.TimeToApproval != nil {
		t.Fatalf("expected nil TimeToApproval, got %v", *patch.TimeToApproval)
	}
	if patch.TimeToMerge == nil || *patch.TimeToMerge != 3*3600 {
		t.Fatalf("TimeToMerge got %v want 10800", patch.TimeToMerge)
	}
}

