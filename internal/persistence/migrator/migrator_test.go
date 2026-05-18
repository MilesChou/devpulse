package migrator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/migrator"
	"github.com/mileschou/devpulse/migrations"
)

func openMemory(t *testing.T) *persistence.Connection {
	t.Helper()
	conn, err := persistence.Open(context.Background(), "memory")
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	t.Cleanup(func() { _ = conn.DB.Close() })
	return conn
}

func TestMigrator_MigrateUp_AppliesAll(t *testing.T) {
	conn := openMemory(t)
	m := migrator.New(conn.DB, conn.Dialect, migrations.FS, nil)

	if err := m.MigrateUp(context.Background()); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// All four tables should exist.
	for _, table := range []string{"repos", "builds", "pull_requests", "pull_request_reviews"} {
		var n int
		err := conn.DB.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("table %s should be empty, got %d rows", table, n)
		}
	}
}

func countUpMigrations(t *testing.T) int {
	t.Helper()
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		// Skip dialect-specific overrides; they share a timestamp with
		// the generic version which already counted.
		if strings.Count(name, ".") > 2 {
			continue
		}
		n++
	}
	return n
}

func TestMigrator_MigrateUp_Idempotent(t *testing.T) {
	conn := openMemory(t)
	m := migrator.New(conn.DB, conn.Dialect, migrations.FS, nil)

	ctx := context.Background()
	if err := m.MigrateUp(ctx); err != nil {
		t.Fatalf("first up: %v", err)
	}
	if err := m.MigrateUp(ctx); err != nil {
		t.Fatalf("second up should be no-op, got: %v", err)
	}

	versions, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	want := countUpMigrations(t)
	if len(versions) != want {
		t.Fatalf("expected %d applied versions, got %d (%v)", want, len(versions), versions)
	}
}

func TestMigrator_MigrateDown_RollsBackOneStep(t *testing.T) {
	conn := openMemory(t)
	m := migrator.New(conn.DB, conn.Dialect, migrations.FS, nil)
	ctx := context.Background()

	if err := m.MigrateUp(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}

	if err := m.MigrateDown(ctx); err != nil {
		t.Fatalf("down: %v", err)
	}

	versions, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	want := countUpMigrations(t) - 1
	if len(versions) != want {
		t.Fatalf("expected %d left after one down, got %d (%v)", want, len(versions), versions)
	}

	// The last (highest timestamp) table should be gone. We don't hard-
	// code its name; instead we walk the migrations FS for the latest
	// .up.sql entity and assert the table is dropped.
	last := highestEntityName(t)
	if _, err := conn.DB.Exec(`SELECT * FROM ` + last); err == nil {
		t.Fatalf("%s should no longer exist after one down", last)
	}
}

// highestEntityName returns the entity in the latest-timestamped up
// migration. Mirrors what Plan(DirDown) picks first.
func highestEntityName(t *testing.T) string {
	t.Helper()
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var best string
	var bestTS string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") || strings.Count(name, ".") > 2 {
			continue
		}
		// Format: <ts>_<entity>.up.sql
		under := strings.IndexByte(name, '_')
		dot := strings.Index(name, ".up.sql")
		if under < 0 || dot < 0 {
			continue
		}
		ts := name[:under]
		entity := name[under+1 : dot]
		if ts > bestTS {
			bestTS = ts
			best = entity
		}
	}
	return best
}

func TestMigrator_MigrateDown_NoOpWhenEmpty(t *testing.T) {
	conn := openMemory(t)
	m := migrator.New(conn.DB, conn.Dialect, migrations.FS, nil)
	if err := m.MigrateDown(context.Background()); err != nil {
		t.Fatalf("down on empty state should be no-op, got: %v", err)
	}
}
