package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
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

func TestServe_NotImplemented(t *testing.T) {
	setEnv(t)
	out, err := runCmd(t, "serve")
	if err != nil {
		t.Fatalf("serve placeholder should not error: %v", err)
	}
	if !strings.Contains(out, "not implemented") {
		t.Fatalf("expected stub message, got: %q", out)
	}
}
