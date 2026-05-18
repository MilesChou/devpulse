// Package migrator runs schema migrations against any dialect supported by
// internal/persistence/dialect. Migration files follow the hydra-style
// layout (timestamp, optional dialect segment, up/down) so a single
// migrations directory powers PostgreSQL, MySQL, and SQLite.
//
// The migrator state lives in a `schema_migrations` table: one row per
// applied timestamp. MigrateUp is idempotent; running it twice does
// nothing on the second call. MigrateDown rolls back one step at a time.
package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"

	"github.com/mileschou/devpulse/internal/persistence/dialect"
)

// Migrator runs migrations against a database/sql DB.
type Migrator struct {
	db      *sql.DB
	dialect dialect.Dialect
	source  fs.FS
	logger  *slog.Logger
}

// New constructs a Migrator. The caller owns the *sql.DB lifecycle.
func New(db *sql.DB, d dialect.Dialect, source fs.FS, logger *slog.Logger) *Migrator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Migrator{db: db, dialect: d, source: source, logger: logger}
}

// ensureTable creates schema_migrations if it does not exist. We use a
// portable DDL: a single BIGINT version column. We deliberately do not
// store an `applied_at` timestamp here to keep the cross-dialect SQL
// trivial; the table is a journal, not an audit log.
func (m *Migrator) ensureTable(ctx context.Context) error {
	ddl := `CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY)`
	_, err := m.db.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("migrator: ensure schema_migrations: %w", err)
	}
	return nil
}

// applied loads the set of applied versions.
func (m *Migrator) applied(ctx context.Context) (map[int64]struct{}, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrator: load applied: %w", err)
	}
	defer rows.Close()

	out := map[int64]struct{}{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = struct{}{}
	}
	return out, rows.Err()
}

// MigrateUp applies every pending up migration in ascending timestamp order.
func (m *Migrator) MigrateUp(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}

	plan, err := Plan(m.source, m.dialect, DirUp)
	if err != nil {
		return err
	}
	applied, err := m.applied(ctx)
	if err != nil {
		return err
	}

	for _, step := range plan {
		if _, done := applied[step.Timestamp]; done {
			continue
		}
		m.logger.Info("applying migration", slog.Int64("version", step.Timestamp), slog.String("file", step.Path))

		if err := m.runStep(ctx, step, true); err != nil {
			return fmt.Errorf("migrator: apply %d (%s): %w", step.Timestamp, step.Path, err)
		}
	}
	return nil
}

// MigrateDown rolls back one applied migration. If nothing is applied, it
// is a no-op and returns nil.
func (m *Migrator) MigrateDown(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}

	plan, err := Plan(m.source, m.dialect, DirDown)
	if err != nil {
		return err
	}
	applied, err := m.applied(ctx)
	if err != nil {
		return err
	}

	for _, step := range plan { // plan is sorted desc for down
		if _, done := applied[step.Timestamp]; !done {
			continue
		}
		m.logger.Info("rolling back migration", slog.Int64("version", step.Timestamp), slog.String("file", step.Path))

		if err := m.runStep(ctx, step, false); err != nil {
			return fmt.Errorf("migrator: rollback %d (%s): %w", step.Timestamp, step.Path, err)
		}
		return nil // one step per invocation
	}
	return nil
}

// Status returns applied versions, ordered ascending.
func (m *Migrator) Status(ctx context.Context) ([]int64, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}
	applied, err := m.applied(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(applied))
	for v := range applied {
		out = append(out, v)
	}
	slices.Sort(out)
	return out, nil
}

func (m *Migrator) runStep(ctx context.Context, step Step, up bool) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, step.SQL); err != nil {
		return err
	}

	if up {
		_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (`+m.dialect.Placeholder(1)+`)`, step.Timestamp)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = `+m.dialect.Placeholder(1), step.Timestamp)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}
