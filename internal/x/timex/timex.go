// Package timex hosts tiny time.Time helpers shared by adapters.
package timex

import "time"

// PtrUTC returns a pointer to t.UTC(), preserving nil input.
// Useful for converting upstream JSON timestamps into a model's *time.Time
// field while normalizing to UTC.
func PtrUTC(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
