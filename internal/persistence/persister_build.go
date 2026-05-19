package persistence

import (
	"context"
	"fmt"

	"github.com/mileschou/devpulse/internal/build"
	"github.com/mileschou/devpulse/internal/fetching"
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

// ListMissingAuthorSHAs returns SHAs whose author_account is NULL in the
// given month, deduplicated.
func (b *BuildPersister) ListMissingAuthorSHAs(ctx context.Context, repoID string, month fetching.MonthRange) ([]commitsha.SHA, error) {
	const q = `SELECT DISTINCT commit_sha FROM builds
	            WHERE repo_id = ?
	              AND started_at >= ?
	              AND started_at <  ?
	              AND author_account IS NULL`

	rows, err := b.QueryCtx(ctx, q, repoID, month.Start, month.End)
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
