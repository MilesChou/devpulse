package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/x/commitsha"
)

// BuildPersister persists CI builds. Implements fetching.BuildWriter.
type BuildPersister struct{ *Persister }

func NewBuildPersister(p *Persister) *BuildPersister { return &BuildPersister{Persister: p} }

// UpsertMany inserts each build, ignoring duplicates by (repo_id, external_id).
// Returns the number of rows actually inserted. Updates are not currently
// performed — builds are immutable once recorded by the upstream CI provider.
func (b *BuildPersister) UpsertMany(ctx context.Context, repoID string, builds []build.Build) (int, error) {
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

		res, err := tx.ExecContext(ctx, insert,
			id,
			repoID,
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
			row.Status == build.StatusFailed || row.Status == build.StatusErrored,
			row.Started(),
			row.DurationSeconds(),
			"{}", // raw_payload placeholder until upstream wires it through
			now,
			now,
		)
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

// MaxStartedAt returns the largest started_at persisted for the repo,
// along with a `has` flag distinguishing "empty store" from a stored
// MAX that happens to be the zero time.
//
// The query is index-only on (repo_id, started_at) — see the
// `builds_repo_started_idx` declared in 20260518000002_builds.up.sql —
// so this is cheap enough for the orchestrator to call before every
// incremental sync.
//
// Returns (zero, false, nil) when the repo has no builds yet; the
// orchestrator treats that as a cold-start signal and back-fills the
// full upstream history.
func (b *BuildPersister) MaxStartedAt(ctx context.Context, repoID string) (time.Time, bool, error) {
	const q = `SELECT MAX(started_at) FROM builds WHERE repo_id = ?`

	// MAX() is an aggregate so different drivers return the value with
	// different concrete types:
	//   - pgx (PostgreSQL): time.Time
	//   - go-sql-driver/mysql: time.Time (with parseTime=true) or []byte
	//   - modernc.org/sqlite: string, because the aggregate result has
	//     no declared TIMESTAMP affinity to trigger its auto-parser
	// Scanning into `any` lets us handle each via a type switch instead
	// of dialect-specific SELECT casts. NULL becomes the nil case, which
	// is the cold-start signal.
	var raw any
	if err := b.QueryRowCtx(ctx, q, repoID).Scan(&raw); err != nil {
		return time.Time{}, false, fmt.Errorf("build max started_at: %w", err)
	}
	switch v := raw.(type) {
	case nil:
		return time.Time{}, false, nil
	case time.Time:
		return v.UTC(), true, nil
	case string:
		t, err := parseDBTimestamp(v)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("build max started_at parse %q: %w", v, err)
		}
		return t, true, nil
	case []byte:
		t, err := parseDBTimestamp(string(v))
		if err != nil {
			return time.Time{}, false, fmt.Errorf("build max started_at parse %q: %w", string(v), err)
		}
		return t, true, nil
	default:
		return time.Time{}, false, fmt.Errorf("build max started_at: unexpected driver type %T", v)
	}
}

// parseDBTimestamp accepts the wire formats every driver we support
// emits for a TIMESTAMP column. RFC3339Nano covers PostgreSQL's text
// fallback and most JDBC-style outputs — `.999999999` is variable
// length so it also matches strings with no fractional seconds, which
// is why plain RFC3339 is not listed separately. The `-0700 MST`
// variant is what modernc.org/sqlite stores when given a Go time.Time
// (it falls back to fmt.Sprint, i.e. time.Time.String()); the bare
// `2006-01-02 15:04:05` variants are MySQL DATETIME / TIMESTAMP.
//
// TODO: this helper has nothing build-specific in it — promote it to
// a package-level timex helper as soon as a second SELECT MAX(<ts>)
// caller shows up (e.g. PR sync gaining a started_at watermark).
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
	cols := `id, repo_id, external_id, commit_sha, author_account, pr_number,
	         status, trigger_event, branch,
	         is_post_merge, is_pull_request, is_deploy_event, is_failure,
	         started_at, duration_seconds, raw_payload,
	         created_at, updated_at`

	values := `?, ?, ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?, ?, ?,
	           ?, ?, ?,
	           ?, ?`

	if b.Dialect.IsMySQL() {
		return `INSERT IGNORE INTO builds (` + cols + `) VALUES (` + values + `)`
	}
	return `INSERT INTO builds (` + cols + `) VALUES (` + values + `)
	        ON CONFLICT (repo_id, external_id) DO NOTHING`
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
