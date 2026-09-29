package persistence

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mileschou/devpulse/internal/incident"
)

// IncidentPersister implements fetching.IncidentWriter.
type IncidentPersister struct{ *Persister }

func NewIncidentPersister(p *Persister) *IncidentPersister {
	return &IncidentPersister{Persister: p}
}

// ReplaceForRepo makes the repo's stored incidents equal to incs, in one
// transaction: every existing row for the repo is deleted, then incs are
// inserted. Mirroring (rather than upserting) is what lets a removed or
// renamed incident label take effect without tombstones; incident
// volume is small enough that rewriting the set each sync is cheap.
//
// Returns the number of rows inserted.
func (r *IncidentPersister) ReplaceForRepo(ctx context.Context, repoID string, incs []incident.Incident) (int, error) {
	const del = `DELETE FROM incidents WHERE repo_id = ?`
	const ins = `INSERT INTO incidents
	             (id, repo_id, source, number, title, opened_at, resolved_at, created_at, updated_at)
	             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("incident replace begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	r.Logger.Debug("sql.exec", slog.String("query", del), slog.Any("args", []any{repoID}))
	if _, err := tx.ExecContext(ctx, r.Rebind(del), repoID); err != nil {
		return 0, fmt.Errorf("incident replace delete: %w", err)
	}

	insert := r.Rebind(ins)
	for i := range incs {
		inc := &incs[i]
		inc.ID = r.NewID()
		inc.RepoID = repoID
		now := r.Now()

		args := []any{inc.ID, repoID, inc.Source, inc.Number, inc.Title, inc.OpenedAt, inc.ResolvedAt, now, now}
		r.Logger.Debug("sql.exec", slog.String("query", ins), slog.Any("args", args))
		if _, err := tx.ExecContext(ctx, insert, args...); err != nil {
			return 0, fmt.Errorf("incident replace insert #%d: %w", inc.Number, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("incident replace commit: %w", err)
	}
	return len(incs), nil
}

// List returns every stored incident for the repo, ordered by number.
func (r *IncidentPersister) List(ctx context.Context, repoID string) ([]incident.Incident, error) {
	const q = `SELECT id, repo_id, source, number, title, opened_at, resolved_at
	           FROM incidents WHERE repo_id = ? ORDER BY number`
	return queryIncidents(ctx, r.Persister, q, repoID)
}

func queryIncidents(ctx context.Context, p *Persister, q string, args ...any) ([]incident.Incident, error) {
	rows, err := p.QueryCtx(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("incident query: %w", err)
	}
	defer rows.Close()

	var out []incident.Incident
	for rows.Next() {
		var inc incident.Incident
		if err := rows.Scan(
			&inc.ID, &inc.RepoID, &inc.Source, &inc.Number, &inc.Title,
			&inc.OpenedAt, &inc.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("incident scan: %w", err)
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}
