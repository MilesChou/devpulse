package persistence

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// BuildPersister persists CI builds. Implements fetching.BuildWriter.
type BuildPersister struct{ *Persister }

func NewBuildPersister(p *Persister) *BuildPersister { return &BuildPersister{Persister: p} }

// UpsertMany inserts each build, ignoring duplicates by
// (repo_id, ci_provider, external_id) — external IDs are only unique
// within one provider, so the provider name is part of the dedupe key.
// Returns the number of rows actually inserted. Updates are not currently
// performed — builds are immutable once recorded by the upstream CI provider.
func (b *BuildPersister) UpsertMany(ctx context.Context, repoID, ciProvider string, builds []build.Build) (int, error) {
	if len(builds) == 0 {
		return 0, nil
	}

	insert := b.Rebind(b.buildInsertSQL())

	tx, err := b.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("build upsert: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var written int
	for i := range builds {
		row := builds[i]
		id := row.ID
		if id == "" {
			id = b.NewID()
		}
		now := b.Now()

		args := []any{
			id,
			repoID,
			ciProvider,
			row.ExternalID,
			row.CommitSHA.String(),
			nullStr(row.Author),
			nullInt(row.PRNumber),
			row.Status.String(),
			row.Trigger.String(),
			nullStr(row.Branch),
			false, // is_post_merge — populated later by post-processing
			row.Trigger == build.TriggerPullRequest,
			false, // is_deploy_event
			row.Status.IsFailure(),
			row.Started(),
			row.DurationSeconds(),
			"{}", // raw_payload placeholder until upstream wires it through
			now,
			now,
		}
		b.Logger.Debug("sql.exec", slog.String("query", b.buildInsertSQL()), slog.Any("args", args))
		res, err := tx.ExecContext(ctx, insert, args...)
		if err != nil {
			return written, fmt.Errorf("build upsert row: %w", err)
		}
		n, _ := res.RowsAffected()
		written += int(n)
	}

	if err := tx.Commit(); err != nil {
		return written, fmt.Errorf("build upsert commit: %w", err)
	}
	return written, nil
}

// MaxStartedAt returns the largest started_at for the repo and CI
// provider (UTC). The watermark is scoped per provider so one
// provider's progress never advances another's cursor.
// Index-only on builds_repo_provider_started_idx. Returns
// (zero, false, nil) when the provider has no rows yet — the
// orchestrator's cold-start signal.
func (b *BuildPersister) MaxStartedAt(ctx context.Context, repoID, ciProvider string) (time.Time, bool, error) {
	const q = `SELECT MAX(started_at) FROM builds WHERE repo_id = ? AND ci_provider = ?`

	// MAX() has no declared TIMESTAMP affinity so drivers disagree on
	// the return type: time.Time for pgx and mysql (with parseTime),
	// string for modernc.org/sqlite, []byte for some MySQL configs.
	// Scan into `any` and dispatch.
	var raw any
	if err := b.QueryRowCtx(ctx, q, repoID, ciProvider).Scan(&raw); err != nil {
		return time.Time{}, false, fmt.Errorf("build max started_at: %w", err)
	}
	// nil is the "empty store" signal (no rows); every other driver
	// type is dispatched by the shared anyToTime helper.
	if raw == nil {
		return time.Time{}, false, nil
	}
	t, err := anyToTime(raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("build max started_at: %w", err)
	}
	return t, true, nil
}

// parseDBTimestamp tries the wire formats each driver emits for
// TIMESTAMP: RFC3339Nano (PostgreSQL / JDBC; the `.999999999` also
// matches strings with no fractional), Go's time.String() form
// (modernc.org/sqlite), and bare datetime (MySQL).
//
// TODO: promote to a package-level timex helper once a second
// SELECT MAX(<timestamp>) caller appears.
func parseDBTimestamp(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("no matching layout")
}

// ListMissingAuthorSHAs returns SHAs whose author_account is NULL,
// deduplicated.
func (b *BuildPersister) ListMissingAuthorSHAs(ctx context.Context, repoID string) ([]commitsha.SHA, error) {
	const q = `SELECT DISTINCT commit_sha FROM builds
	            WHERE repo_id = ?
	              AND author_account IS NULL`

	rows, err := b.QueryCtx(ctx, q, repoID)
	if err != nil {
		return nil, fmt.Errorf("list missing shas: %w", err)
	}
	defer rows.Close()

	out := make([]commitsha.SHA, 0, 16)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		sha, err := commitsha.Parse(raw)
		if err != nil {
			// Skip malformed rows but don't fail the whole list.
			continue
		}
		out = append(out, sha)
	}
	return out, rows.Err()
}

// UpdateAuthorBySHA writes login to every build row matching (repo_id, sha).
func (b *BuildPersister) UpdateAuthorBySHA(ctx context.Context, repoID string, sha commitsha.SHA, login string) error {
	const q = `UPDATE builds SET author_account = ?, updated_at = ? WHERE repo_id = ? AND commit_sha = ?`
	_, err := b.ExecCtx(ctx, q, login, b.Now(), repoID, sha.String())
	if err != nil {
		return fmt.Errorf("update author: %w", err)
	}
	return nil
}

// buildInsertSQL returns the right INSERT for the dialect.
//
// PostgreSQL/SQLite support ON CONFLICT; MySQL uses INSERT IGNORE.
// All three skip duplicates by (repo_id, external_id).
func (b *BuildPersister) buildInsertSQL() string {
	cols := `id, repo_id, ci_provider, external_id, commit_sha, author_account, pr_number,
	         status, trigger_event, branch,
	         is_post_merge, is_pull_request, is_deploy_event, is_failure,
	         started_at, duration_seconds, raw_payload,
	         created_at, updated_at`

	values := `?, ?, ?, ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?`

	if b.Dialect.IsMySQL() {
		return `INSERT IGNORE INTO builds (` + cols + `) VALUES (` + values + `)`
	}
	return `INSERT INTO builds (` + cols + `) VALUES (` + values + `)
	        ON CONFLICT (repo_id, ci_provider, external_id) DO NOTHING`
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
