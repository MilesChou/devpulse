package pullrequest

import (
	"testing"
	"time"
)

func TestIsPlausibleCommitTime(t *testing.T) {
	cases := []struct {
		in   time.Time
		want bool
	}{
		{time.Unix(0, 0).UTC(), false},
		{time.Date(1970, 6, 1, 0, 0, 0, 0, time.UTC), false},
		{MinCommitTime, true},
		{time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), true},
	}
	for _, c := range cases {
		if got := IsPlausibleCommitTime(c.in); got != c.want {
			t.Errorf("IsPlausibleCommitTime(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestClampFirstCommitAt(t *testing.T) {
	merged := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)
	before := merged.Add(-time.Hour)
	after := time.Date(2106, 2, 7, 0, 0, 0, 0, time.UTC)

	if got := ClampFirstCommitAt(nil, merged); got != nil {
		t.Fatalf("nil: got %v", got)
	}
	if got := ClampFirstCommitAt(&before, merged); got == nil || !got.Equal(before) {
		t.Fatalf("before merge: got %v, want %v", got, before)
	}
	if got := ClampFirstCommitAt(&after, merged); got == nil || !got.Equal(merged) {
		t.Fatalf("after merge: got %v, want %v", got, merged)
	}
}
