package cli

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func setEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DEVPULSE_DSN", "memory")
	t.Setenv("LOG_LEVEL", "error") // keep test output quiet
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
}

// setEnvSharedSQLite points DEVPULSE_DSN at a temp-file SQLite so
// multiple runCmd calls in the same test see the same DB. Use this when
// a test needs to chain commands (e.g. `repo add` then
// `repo set-pr-start`) — the bare "memory" DSN gives each connection
// its own in-process DB, which doesn't survive across runCmd. Memory
// DSN auto-migrates; the file DSN does not, so the caller must run
// `migrate up` first.
func setEnvSharedSQLite(t *testing.T) {
	t.Helper()
	t.Setenv("DEVPULSE_DSN", "sqlite://"+t.TempDir()+"/devpulse.db")
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
}

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	prev := SetStdout(&buf)
	defer SetStdout(prev)

	root := NewRootCmd()
	root.SetArgs(args)
	root.SetOut(&buf)
	root.SetErr(&buf)
	err := root.ExecuteContext(context.Background())
	return buf.String(), err
}

func TestRepoAdd_RejectsInvalidName(t *testing.T) {
	setEnv(t)
	_, err := runCmd(t, "repo", "add", "not-a-slug")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMigrateUp_OnMemoryDSN(t *testing.T) {
	setEnv(t)
	out, err := runCmd(t, "migrate", "up")
	if err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if !strings.Contains(out, "migrations up: ok") {
		t.Fatalf("output: %q", out)
	}
}

func TestServe_RefusesPublicAddrWithoutToken(t *testing.T) {
	setEnv(t)
	t.Setenv("HTTP_ADDR", "0.0.0.0:0")
	t.Setenv("DEVPULSE_API_TOKEN", "")
	_, err := runCmd(t, "serve")
	if err == nil || !strings.Contains(err.Error(), "DEVPULSE_API_TOKEN") {
		t.Fatalf("expected a missing-token error, got %v", err)
	}
}

// syncBuffer lets the test read serve's output while serve is still
// writing it from another goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestServe_StopsOnCancel(t *testing.T) {
	setEnv(t)
	t.Setenv("HTTP_ADDR", "127.0.0.1:0")
	t.Setenv("DEVPULSE_API_TOKEN", "")

	var out syncBuffer
	prev := SetStdout(&out)
	defer SetStdout(prev)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := NewRootCmd()
	root.SetArgs([]string{"serve"})
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()

	deadline := time.After(10 * time.Second)
	for !strings.Contains(out.String(), "serving on") {
		select {
		case err := <-done:
			t.Fatalf("serve exited early: %v (output %q)", err, out.String())
		case <-deadline:
			t.Fatalf("serve did not start: %q", out.String())
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop on cancel")
	}
	if !strings.Contains(out.String(), "server stopped") {
		t.Fatalf("output: %q", out.String())
	}
}

func TestRepoRemove(t *testing.T) {
	setEnvSharedSQLite(t)
	if _, err := runCmd(t, "migrate", "up"); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if _, err := runCmd(t, "repo", "add", "MilesChou/devpulse"); err != nil {
		t.Fatalf("repo add: %v", err)
	}

	if _, err := runCmd(t, "repo", "remove", "MilesChou/devpulse"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("remove without --yes must refuse, got %v", err)
	}
	out, err := runCmd(t, "repo", "remove", "MilesChou/devpulse", "--yes")
	if err != nil || !strings.Contains(out, "Removed MilesChou/devpulse") {
		t.Fatalf("remove: %v (%q)", err, out)
	}
	if _, err := runCmd(t, "repo", "remove", "MilesChou/devpulse", "--yes"); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("remove twice: want not-registered error, got %v", err)
	}
}
