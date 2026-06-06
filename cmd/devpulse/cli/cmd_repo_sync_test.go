package cli

import (
	"strings"
	"testing"
)

func TestRepoSync_RequiresGitHubToken(t *testing.T) {
	setEnv(t)
	// Neither token set.
	out, err := runCmd(t, "repo", "sync", "MilesChou/devpulse")
	if err == nil {
		t.Fatalf("expected error when GITHUB_TOKEN missing, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("expected GITHUB_TOKEN in error, got: %v", err)
	}
}

func TestRepoSync_RejectsInvalidRepoName(t *testing.T) {
	setEnv(t)
	t.Setenv("GITHUB_TOKEN", "fake-gh-token")
	_, err := runCmd(t, "repo", "sync", "not-a-slug")
	if err == nil {
		t.Fatalf("expected error for invalid repo name")
	}
	if !strings.Contains(err.Error(), "invalid repo") {
		t.Fatalf("expected `invalid repo` in error, got: %v", err)
	}
}

func TestRepoSync_RejectsMissingArg(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "repo", "sync")
	if err == nil {
		t.Fatalf("expected error when owner/name arg is missing")
	}
}

// TestRepoSync_TokenCheckRunsAfterRepoParse pins the validation order: repo
// parsing fires before token checks, so an obviously-malformed repo argument
// errors with `invalid repo` rather than a token error — easier for users
// to spot the real bug. This locks in the fail-fast contract documented in
// the command Long help.
func TestRepoSync_TokenCheckRunsAfterRepoParse(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "repo", "sync", "not-a-slug")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid repo") {
		t.Fatalf("expected repo parse error to take precedence over token error, got: %v", err)
	}
}
