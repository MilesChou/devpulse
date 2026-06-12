package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mileschou/devpulse/internal/repo"
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

// TestSync_EmptyStoreIsNoop verifies that running `devpulse sync` on a
// fresh store does NOT try to contact GitHub / Travis (so it does not
// fail just because tokens are fake). This pins the "no repos → graceful
// no-op" contract — important when the command is wired into cron, so
// it doesn't error on first boot before any `repo add`.
func TestSync_EmptyStoreIsNoop(t *testing.T) {
	setEnv(t)
	t.Setenv("GITHUB_TOKEN", "fake-gh-token")

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

// TestSyncRepos_StopsOnCancelledContext locks in the Ctrl-C contract:
// if the context is already cancelled when the loop reaches the next
// repo, syncRepos must bail out with the ctx error instead of marching
// through the remaining repos producing one `failed ...: context
// canceled` line per row.
//
// We call syncRepos directly (not the runSync entry point) because
// runSync calls buildDeps, which for memory DSNs opens a fresh empty
// DB — so any repos we seeded on a different deps would not be visible.
func TestSyncRepos_StopsOnCancelledContext(t *testing.T) {
	setEnv(t)
	t.Setenv("GITHUB_TOKEN", "fake-gh-token")

	ctx := context.Background()
	d, err := buildDeps(ctx)
	if err != nil {
		t.Fatalf("buildDeps: %v", err)
	}
	defer d.close(ctx)

	// Two repos so we'd notice if the loop kept going after cancellation.
	repos := make([]repo.Repo, 0, 2)
	for _, name := range []string{"alpha/one", "beta/two"} {
		fn, err := repo.ParseFullName(name)
		if err != nil {
			t.Fatalf("parse %q: %v", name, err)
		}
		r, err := d.repos.EnsureID(ctx, "github", fn)
		if err != nil {
			t.Fatalf("seed %q: %v", name, err)
		}
		repos = append(repos, r)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	var buf bytes.Buffer
	err = syncRepos(cancelled, d, &buf, repos)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "interrupted") {
		t.Fatalf("expected interrupted notice in output, got: %q", out)
	}
	// The smoke test: no PR-sync progress line should have been printed,
	// which would only show up if the loop had entered syncOneRepo.
	if strings.Contains(out, "pull requests:") {
		t.Fatalf("expected no syncOneRepo work, got: %q", out)
	}
	// The summary line is only printed when the loop finishes naturally;
	// an interrupted run should NOT print it.
	if strings.Contains(out, "sync: synced=") {
		t.Fatalf("expected no summary line on interrupted run, got: %q", out)
	}
}
