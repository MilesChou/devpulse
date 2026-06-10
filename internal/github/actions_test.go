package github

import (
	"testing"

	"github.com/mileschou/devpulse/internal/build"
)

// TestResolveRunStatus covers the GitHub Actions conclusion → build.Status
// mapping. timed_out and startup_failure are the load-bearing cases: they
// must satisfy build.Status.IsFailure or the build-failure-rate metric
// undercounts for Actions repos.
func TestResolveRunStatus(t *testing.T) {
	tests := []struct {
		name       string
		conclusion *string
		want       build.Status
	}{
		{"nil", nil, build.StatusUnknown},
		{"success", new("success"), build.StatusPassed},
		{"failure", new("failure"), build.StatusFailed},
		{"timed_out", new("timed_out"), build.StatusErrored},
		{"startup_failure", new("startup_failure"), build.StatusErrored},
		{"cancelled", new("cancelled"), build.StatusCanceled},
		{"neutral", new("neutral"), build.StatusUnknown},
		{"skipped", new("skipped"), build.StatusUnknown},
		{"stale", new("stale"), build.StatusUnknown},
		{"action_required", new("action_required"), build.StatusUnknown},
		{"unrecognized", new("something_new"), build.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveRunStatus(tt.conclusion); got != tt.want {
				t.Errorf("resolveRunStatus(%v) = %v, want %v", tt.conclusion, got, tt.want)
			}
		})
	}
}
