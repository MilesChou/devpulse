package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mileschou/devpulse/internal/dora"
)

// DORAInput loads everything dora.Compute needs for [from, to):
//
//   - deployments: PRs merged into defaultBranch with merged_at in the
//     window. A revert deployment also carries the merge time of the PR
//     it reverts, which may lie before the window.
//   - incidents resolved in the window.
//
// Classification (revert / hotfix) is left to dora.Compute so a
// hotfix-label change applies without a re-sync. The returned Input has
// no HotfixLabel; the caller sets it from the repo settings.
func (m *MetricsPersister) DORAInput(ctx context.Context, repoID, defaultBranch string, from, to time.Time) (dora.Input, error) {
	in := dora.Input{From: from, To: to}

	deps, err := m.doraDeployments(ctx, repoID, defaultBranch, from, to)
	if err != nil {
		return dora.Input{}, err
	}
	in.Deployments = deps

	const iq = `SELECT id, repo_id, source, number, title, opened_at, resolved_at
	            FROM incidents
	            WHERE repo_id = ? AND resolved_at >= ? AND resolved_at < ?
	            ORDER BY resolved_at`
	incs, err := queryIncidents(ctx, m.Persister, iq, repoID, from, to)
	if err != nil {
		return dora.Input{}, fmt.Errorf("dora incidents: %w", err)
	}
	in.Incidents = incs
	return in, nil
}

func (m *MetricsPersister) doraDeployments(ctx context.Context, repoID, defaultBranch string, from, to time.Time) ([]dora.Deployment, error) {
	const q = `SELECT number, merged_at, first_commit_at, title, labels, head_ref, reverts_number
	           FROM pull_requests
	           WHERE repo_id = ? AND status = 'merged' AND base_ref = ?
	             AND merged_at >= ? AND merged_at < ?
	           ORDER BY merged_at`

	rows, err := m.QueryCtx(ctx, q, repoID, defaultBranch, from, to)
	if err != nil {
		return nil, fmt.Errorf("dora deployments: %w", err)
	}
	defer rows.Close()

	var out []dora.Deployment
	for rows.Next() {
		var (
			d                      dora.Deployment
			mergedAt               *time.Time
			title, labels, headRef sql.NullString
		)
		if err := rows.Scan(&d.Number, &mergedAt, &d.FirstCommitAt, &title, &labels, &headRef, &d.RevertsNumber); err != nil {
			return nil, fmt.Errorf("dora deployments scan: %w", err)
		}
		if mergedAt == nil {
			continue // unreachable given the WHERE clause; keeps the deref safe
		}
		d.MergedAt = mergedAt.UTC()
		d.Title = title.String
		d.Labels = decodeLabels(labels.String)
		d.HeadRef = headRef.String
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dora deployments rows: %w", err)
	}

	// Resolve reverted-PR merge times after the cursor is closed: some
	// drivers (SQLite with a single connection) cannot run a second
	// query while rows are still open.
	for i := range out {
		if out[i].RevertsNumber == nil {
			continue
		}
		at, err := m.mergedAtByNumber(ctx, repoID, *out[i].RevertsNumber)
		if err != nil {
			return nil, err
		}
		out[i].RevertedMergedAt = at
	}
	return out, nil
}

// mergedAtByNumber returns the merge time of PR #number, or nil when the
// PR is unknown or was never merged.
func (m *MetricsPersister) mergedAtByNumber(ctx context.Context, repoID string, number int) (*time.Time, error) {
	const q = `SELECT merged_at FROM pull_requests WHERE repo_id = ? AND number = ?`

	var at *time.Time
	err := m.QueryRowCtx(ctx, q, repoID, number).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dora reverted pr #%d: %w", number, err)
	}
	if at != nil {
		u := at.UTC()
		at = &u
	}
	return at, nil
}
