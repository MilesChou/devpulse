package travis

import (
	"testing"

	"github.com/mileschou/devpulse/internal/build"
)

// TestResolveStatus mirrors the GitHub adapter's TestResolveRunStatus: both
// providers feed the same build-failure-rate metric, so `errored` must
// satisfy build.Status.IsFailure.
func TestResolveStatus(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  build.Status
	}{
		{"passed", "passed", build.StatusPassed},
		{"failed", "failed", build.StatusFailed},
		{"errored", "errored", build.StatusErrored},
		{"canceled", "canceled", build.StatusCanceled},
		{"case-insensitive", "Errored", build.StatusErrored},
		{"in-flight started", "started", build.StatusUnknown},
		{"in-flight created", "created", build.StatusUnknown},
		{"empty", "", build.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveStatus(tt.state); got != tt.want {
				t.Errorf("resolveStatus(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}
