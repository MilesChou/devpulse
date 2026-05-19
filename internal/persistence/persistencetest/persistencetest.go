// Package persistencetest hosts shared test helpers for persistence-
// backed integration tests.
//
// By default tests run against in-memory SQLite ("memory" DSN). Set the
// DEVPULSE_DSN environment variable to redirect to any supported dialect
// — this is what the CI matrix uses to run the same suite against
// PostgreSQL and MySQL too.
//
// When pointed at a non-memory DSN, every test starts by rolling
// migrations back to an empty schema and re-applying them. Tests sharing
// one database therefore MUST run serially: `go test -p 1`.
package persistencetest

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/dsn"
	"github.com/mileschou/devpulse/internal/persistence/migrator"
	"github.com/mileschou/devpulse/migrations"
)

// NewMemoryPersister opens the dialect under test, applies every
// migration, and returns the Persister. The *sql.DB is registered with
// t.Cleanup so callers do not need to close it themselves.
//
// The DSN is read from DEVPULSE_DSN; it defaults to "memory" for the
// developer-laptop case.
func NewMemoryPersister(t *testing.T) *persistence.Persister {
	t.Helper()
	ctx := context.Background()

	raw := strings.TrimSpace(os.Getenv("DEVPULSE_DSN"))
	if raw == "" {
		raw = "memory"
	}

	conn, err := persistence.Open(ctx, raw)
	if err != nil {
		t.Fatalf("persistencetest: open %s: %v", raw, err)
	}
	t.Cleanup(func() { _ = conn.DB.Close() })

	// A persistent DB (file SQLite, remote Postgres/MySQL) carries schema
	// and rows from previous test runs. Roll migrations back first so
	// every test starts identical to a fresh in-memory instance.
	m := migrator.New(conn.DB, conn.Dialect, migrations.FS, nil)
	if !dsn.IsMemory(raw) {
		for {
			before, _ := m.Status(ctx)
			if err := m.MigrateDown(ctx); err != nil {
				t.Fatalf("persistencetest: pre-test migrate down: %v", err)
			}
			after, _ := m.Status(ctx)
			if len(after) == len(before) {
				break
			}
		}
	}
	if err := m.MigrateUp(ctx); err != nil {
		t.Fatalf("persistencetest: migrate up: %v", err)
	}
	return persistence.New(conn, nil)
}
