package cli

import (
	"strings"
	"testing"
)

func TestSync_RequiresGitHubToken(t *testing.T) {
	setEnv(t)
	// Neither token set.
	out, err := runCmd(t, "sync")
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
	out, err := runCmd(t, "sync")
	if err == nil {
		t.Fatalf("expected error when TRAVIS_TOKEN missing, got output: %q", out)
	}
	if !strings.Contains(err.Error(), "TRAVIS_TOKEN") {
		t.Fatalf("expected TRAVIS_TOKEN in error, got: %v", err)
	}
}

// TestSync_EmptyStoreIsNoop verifies that running `devpulse sync` on a
// fresh store does NOT try to contact GitHub / Travis (so it does not
// fail just because tokens are fake). This pins the "no repos → graceful
// no-op" contract — important when the command is wired into cron, so
// it doesn't error on first boot before any `repo add`.
func TestSync_EmptyStoreIsNoop(t *testing.T) {
	setEnv(t)
	t.Setenv("GITHUB_TOKEN", "fake-gh-token")
	t.Setenv("TRAVIS_TOKEN", "fake-travis-token")

	out, err := runCmd(t, "sync")
	if err != nil {
		t.Fatalf("expected no error on empty store, got: %v (output: %q)", err, out)
	}
	if !strings.Contains(out, "no repos in store") {
		t.Fatalf("expected empty-store notice, got: %q", out)
	}
}

func TestSync_RejectsExtraArgs(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "sync", "MilesChou/devpulse")
	if err == nil {
		t.Fatalf("expected error when extra args passed to top-level sync")
	}
}
