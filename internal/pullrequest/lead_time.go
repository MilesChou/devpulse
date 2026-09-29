package pullrequest

import "time"

// MinCommitTime is the earliest commit author time accepted as a DORA
// lead-time start. Author dates are client-supplied metadata: broken
// clocks and tools produce values such as the Unix epoch itself, which
// are not real authoring times and fall outside the MySQL TIMESTAMP
// range ('1970-01-01 00:00:01' onwards). Earlier dates are ignored.
var MinCommitTime = time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC)

// IsPlausibleCommitTime reports whether an author date may serve as a
// lead-time start (see MinCommitTime).
func IsPlausibleCommitTime(t time.Time) bool {
	return !t.Before(MinCommitTime)
}

// ClampFirstCommitAt bounds a PR's earliest commit author time by its
// merge time. A commit cannot have been authored after the PR merged; a
// later date is clock skew, and clamping keeps the stored value inside
// the range the merge time already occupies (the MySQL TIMESTAMP upper
// bound is 2038-01-19). Returns nil when first is nil.
func ClampFirstCommitAt(first *time.Time, mergedAt time.Time) *time.Time {
	if first == nil {
		return nil
	}
	if first.After(mergedAt) {
		t := mergedAt
		return &t
	}
	return first
}
