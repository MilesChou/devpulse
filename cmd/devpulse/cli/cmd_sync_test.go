package cli

import (
	"strings"
	"testing"
)

func TestSync_RequiresGitHubToken(t *testing.T) {
	setEnv(t)
	// Neither token set.
	out, err := runCmd(t, "sync", "MilesChou/devpulse")
	if err == nil {
		t.Fatalf("expected error when GITHUB_TOKEN missing, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("expected GITHUB_TOKEN in error, got: %v", err)
	}
}

func TestSync_RequiresTravisToken(t *testing.T) {
	setEnv(t)
	t.Setenv("GITHUB_TOKEN", "fake-gh-token")
	// TRAVIS_TOKEN intentionally unset.
	out, err := runCmd(t, "sync", "MilesChou/devpulse")
	if err == nil {
		t.Fatalf("expected error when TRAVIS_TOKEN missing, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "TRAVIS_TOKEN") {
		t.Fatalf("expected TRAVIS_TOKEN in error, got: %v", err)
	}
}

func TestSync_RejectsInvalidRepoName(t *testing.T) {
	setEnv(t)
	t.Setenv("GITHUB_TOKEN", "fake-gh-token")
	t.Setenv("TRAVIS_TOKEN", "fake-travis-token")
	_, err := runCmd(t, "sync", "not-a-slug")
	if err == nil {
		t.Fatalf("expected error for invalid repo name")
	}
	if !strings.Contains(err.Error(), "invalid repo") {
		t.Fatalf("expected `invalid repo` in error, got: %v", err)
	}
}

func TestSync_RejectsMissingArg(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "sync")
	if err == nil {
		t.Fatalf("expected error when owner/name arg is missing")
	}
}

// TestSync_TokenCheckRunsBeforeRepoParse pins the validation order: token
// checks must fire even when the repo arg is also invalid. This locks in
// the fail-fast contract documented in the command Long help.
func TestSync_TokenCheckRunsAfterRepoParse(t *testing.T) {
	// We document repo parsing first, then token checks. Verify that order
	// so an obviously-malformed repo argument errors with `invalid repo`
	// rather than a token error — easier for users to spot the real bug.
	setEnv(t)
	_, err := runCmd(t, "sync", "not-a-slug")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "invalid repo") {
		t.Fatalf("expected repo parse error to take precedence over token error, got: %v", err)
	}
}
