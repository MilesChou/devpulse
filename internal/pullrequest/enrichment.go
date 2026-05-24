package pullrequest

import "time"

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
// because the column is non-negative by CHECK constraint.
func ComputeTimeToApproval(readyAt *time.Time, firstApprovedAt *time.Time) *int {
	return secondsBetween(readyAt, firstApprovedAt)
}

// ComputeTimeToMerge returns seconds from firstApprovedAt to mergedAt,
// with the same nil + clamp behavior as ComputeTimeToApproval.
func ComputeTimeToMerge(firstApprovedAt *time.Time, mergedAt *time.Time) *int {
	return secondsBetween(firstApprovedAt, mergedAt)
}

func secondsBetween(start, end *time.Time) *int {
	if start == nil || end == nil {
		return nil
	}
	d := max(0, int(end.Sub(*start).Seconds()))
	return &d
}
