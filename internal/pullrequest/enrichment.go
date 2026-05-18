package pullrequest

import "time"

// EnrichmentPatch is the bag of fields produced by enriching a PR.
// Every field is intentionally explicit — there is no equivalent of the
// Eloquent fillable mass-assignment dance, so silent field drops are
// impossible: each value either flows or it is a compile error.
type EnrichmentPatch struct {
	Additions         int
	Deletions         int
	TotalChangedLines int
	FirstReviewAt     *time.Time
	FirstApprovedAt   *time.Time
	TimeToApproval    *int // seconds
	TimeToMerge       *int // seconds
}

// ReviewAggregate is the subset of derived facts the enrichment pipeline
// extracts from the (possibly filtered) review list.
type ReviewAggregate struct {
	FirstReviewAt   *time.Time
	FirstApprovedAt *time.Time
}

// AggregateReviews returns the first review-of-any-state and the first
// approval among `reviews`, ignoring submissions before readyAt — draft-
// period reviews must not count toward Pickup/Approval lead-time.
//
// readyAt may be nil, in which case every review is considered.
func AggregateReviews(reviews []Review, readyAt *time.Time) ReviewAggregate {
	var firstReview, firstApproved *time.Time

	for _, r := range reviews {
		if readyAt != nil && r.SubmittedAt.Before(*readyAt) {
			continue
		}

		t := r.SubmittedAt
		if firstReview == nil || t.Before(*firstReview) {
			firstReview = &t
		}
		if r.State == ReviewStateApproved {
			if firstApproved == nil || t.Before(*firstApproved) {
				firstApproved = &t
			}
		}
	}

	return ReviewAggregate{
		FirstReviewAt:   firstReview,
		FirstApprovedAt: firstApproved,
	}
}

// ComputeTimeToApproval returns seconds from readyAt to firstApprovedAt,
// or nil if either anchor is missing. Negative results are clamped to 0
// to honor the unsignedInteger storage contract.
func ComputeTimeToApproval(readyAt *time.Time, firstApprovedAt *time.Time) *int {
	if readyAt == nil || firstApprovedAt == nil {
		return nil
	}
	d := int(firstApprovedAt.Sub(*readyAt).Seconds())
	if d < 0 {
		d = 0
	}
	return &d
}

// ComputeTimeToMerge returns seconds from firstApprovedAt to mergedAt,
// or nil if either anchor is missing. Clamped to 0 like above.
func ComputeTimeToMerge(firstApprovedAt *time.Time, mergedAt *time.Time) *int {
	if firstApprovedAt == nil || mergedAt == nil {
		return nil
	}
	d := int(mergedAt.Sub(*firstApprovedAt).Seconds())
	if d < 0 {
		d = 0
	}
	return &d
}

// BuildEnrichmentPatch assembles every enrichment-derived field for the
// given PR snapshot. The PR's ReadyAt and MergedAt are read for the lead-
// time calculation. additions/deletions come from the upstream detail call.
func BuildEnrichmentPatch(pr PullRequest, additions, deletions int, agg ReviewAggregate) EnrichmentPatch {
	stats := ChangeStats{Additions: additions, Deletions: deletions}
	return EnrichmentPatch{
		Additions:         stats.Additions,
		Deletions:         stats.Deletions,
		TotalChangedLines: stats.Total(),
		FirstReviewAt:     agg.FirstReviewAt,
		FirstApprovedAt:   agg.FirstApprovedAt,
		TimeToApproval:    ComputeTimeToApproval(pr.ReadyAt, agg.FirstApprovedAt),
		TimeToMerge:       ComputeTimeToMerge(agg.FirstApprovedAt, pr.MergedAt),
	}
}
