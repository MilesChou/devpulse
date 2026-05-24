package cli

import (
	"strings"
	"testing"
)

func TestRepoConfigSet_RejectsInvalidName(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "repo", "config", "set", "not-a-slug", "pr-start", "5")
	if err == nil {
		t.Fatalf("expected error for invalid owner/name")
	}
}

func TestRepoConfigSet_RejectsUnknownKey(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "repo", "config", "set", "MilesChou/devpulse", "bogus", "5")
	if err == nil {
		t.Fatalf("expected error for unknown setting key")
	}
	if !strings.Contains(err.Error(), "unknown setting") {
		t.Fatalf("error should mention unknown setting, got: %v", err)
	}
}

func TestRepoConfigSet_PRStart_RejectsNonInteger(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "repo", "config", "set", "MilesChou/devpulse", "pr-start", "abc")
	if err == nil {
		t.Fatalf("expected error for non-integer value")
	}
}

func TestRepoConfigSet_PRStart_RejectsZeroAndBelow(t *testing.T) {
	setEnv(t)
	for _, n := range []string{"0", "-1", "-500"} {
		_, err := runCmd(t, "repo", "config", "set", "MilesChou/devpulse", "pr-start", n)
		if err == nil {
			t.Fatalf("expected error for n=%s", n)
		}
	}
}

func TestRepoConfigSet_RepoNotRegistered(t *testing.T) {
	setEnvSharedSQLite(t)
	if _, err := runCmd(t, "migrate", "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	_, err := runCmd(t, "repo", "config", "set", "ghost/repo", "pr-start", "5")
	if err == nil {
		t.Fatalf("expected error for unregistered repo")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("expected helpful 'not registered' hint, got: %v", err)
	}
}

// TestRepoConfigSet_HappyPath_RoundTripsThroughGet verifies the
// end-to-end CLI contract: add → set → get returns what was set.
func TestRepoConfigSet_HappyPath_RoundTripsThroughGet(t *testing.T) {
	setEnvSharedSQLite(t)
	if _, err := runCmd(t, "migrate", "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if _, err := runCmd(t, "repo", "add", "MilesChou/devpulse"); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	out, err := runCmd(t, "repo", "config", "set", "MilesChou/devpulse", "pr-start", "1500")
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if !strings.Contains(out, "pr-start=1500") {
		t.Fatalf("expected confirmation in stdout, got: %q", out)
	}

	out, err = runCmd(t, "repo", "config", "get", "MilesChou/devpulse", "pr-start")
	if err != nil {
		t.Fatalf("get one: %v", err)
	}
	if strings.TrimSpace(out) != "1500" {
		t.Fatalf("expected '1500', got: %q", out)
	}

	out, err = runCmd(t, "repo", "config", "get", "MilesChou/devpulse")
	if err != nil {
		t.Fatalf("get all: %v", err)
	}
	if !strings.Contains(out, "pr-start=1500") {
		t.Fatalf("expected 'pr-start=1500' in all-keys output, got: %q", out)
	}
}

// TestRepoConfigGet_UnknownKey checks that get also validates the key
// argument before going to the DB.
func TestRepoConfigGet_UnknownKey(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "repo", "config", "get", "MilesChou/devpulse", "bogus")
	if err == nil {
		t.Fatalf("expected error for unknown setting key")
	}
}
