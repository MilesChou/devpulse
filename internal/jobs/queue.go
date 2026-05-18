// Package jobs ships a small DB-backed work queue that runs on every
// dialect DevPulse supports (PostgreSQL / MySQL / SQLite). Jobs are
// (kind, payload) tuples persisted in the `jobs` table.
//
// Lease semantics:
//   - A pending row has status="queued" and available_at <= now.
//   - A worker leases it with a conditional UPDATE that flips status to
//     "processing" and bumps attempts. Race losers see RowsAffected=0
//     and move on.
//   - On success the worker calls MarkDone; on failure MarkFailed reschedules
//     with exponential backoff up to max_attempts, then marks "failed".
//
// The implementation is deliberately small: enough for DevPulse's load,
// without dragging in a full queue dependency. Swap in asynq / river if
// the workload outgrows this.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/mileschou/devpulse/internal/persistence"
)

// Status enumerates the lifecycle states.
type Status string

const (
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// Job is the in-memory representation of a row.
type Job struct {
	ID          string
	Kind        string
	Payload     []byte
	Status      Status
	Attempts    int
	MaxAttempts int
	LastError   string
	AvailableAt time.Time
	LeasedUntil *time.Time
}

// Queue is the central enqueue/dequeue API.
type Queue struct {
	p *persistence.Persister
}

// NewQueue wires a Queue onto a Persister.
func NewQueue(p *persistence.Persister) *Queue { return &Queue{p: p} }

// Enqueue inserts a new job. payload is marshaled with encoding/json so
// callers pass strongly-typed structs without worrying about wire format.
//
// runAt may be zero, in which case the job is available immediately.
// maxAttempts <= 0 defaults to 3.
func (q *Queue) Enqueue(ctx context.Context, kind string, payload any, runAt time.Time, maxAttempts int) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("queue: marshal payload: %w", err)
	}
	id := q.p.NewID()
	now := q.p.Now()
	if runAt.IsZero() {
		runAt = now
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	const insert = `INSERT INTO jobs
	    (id, kind, payload, status, attempts, max_attempts, available_at, leased_until, created_at, updated_at)
	     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if _, err := q.p.ExecCtx(ctx, insert,
		id, kind, string(body), string(StatusQueued), 0, maxAttempts, runAt, nil, now, now,
	); err != nil {
		return "", fmt.Errorf("queue: insert: %w", err)
	}
	return id, nil
}

// Lease atomically picks the oldest available queued job for `kind`,
// flips it to processing, and returns it. Returns (nil, nil) when no work
// is available.
//
// leaseDuration is the window before the row is considered crashed and
// returned to the queue.
func (q *Queue) Lease(ctx context.Context, kind string, leaseDuration time.Duration) (*Job, error) {
	now := q.p.Now()
	leaseUntil := now.Add(leaseDuration)

	// Pick the oldest job we COULD claim. The conditional UPDATE below
	// makes sure we did claim it even under concurrent workers.
	const selectQ = `SELECT id, attempts, max_attempts
	                   FROM jobs
	                  WHERE kind = ?
	                    AND status = ?
	                    AND available_at <= ?
	               ORDER BY available_at
	                  LIMIT 1`

	rows, err := q.p.QueryCtx(ctx, selectQ, kind, string(StatusQueued), now)
	if err != nil {
		return nil, fmt.Errorf("queue: lease select: %w", err)
	}

	var (
		id          string
		attempts    int
		maxAttempts int
	)
	if !rows.Next() {
		rows.Close()
		return nil, nil
	}
	if err := rows.Scan(&id, &attempts, &maxAttempts); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	const claim = `UPDATE jobs
	                  SET status        = ?,
	                      attempts      = attempts + 1,
	                      leased_until  = ?,
	                      updated_at    = ?
	                WHERE id = ? AND status = ?`

	res, err := q.p.ExecCtx(ctx, claim, string(StatusProcessing), leaseUntil, now, id, string(StatusQueued))
	if err != nil {
		return nil, fmt.Errorf("queue: lease claim: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Another worker beat us.
		return nil, nil
	}

	return q.loadByID(ctx, id)
}

// MarkDone marks a leased job as successfully completed.
func (q *Queue) MarkDone(ctx context.Context, id string) error {
	const q1 = `UPDATE jobs SET status = ?, leased_until = NULL, updated_at = ? WHERE id = ?`
	_, err := q.p.ExecCtx(ctx, q1, string(StatusDone), q.p.Now(), id)
	if err != nil {
		return fmt.Errorf("queue: mark done: %w", err)
	}
	return nil
}

// MarkFailed records the error and either reschedules with exponential
// backoff (status back to queued) or burns the job out (status=failed)
// once attempts exceed max_attempts.
func (q *Queue) MarkFailed(ctx context.Context, id string, jobErr error) error {
	job, err := q.loadByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return errors.New("queue: mark failed: job not found")
	}

	now := q.p.Now()
	errText := ""
	if jobErr != nil {
		errText = jobErr.Error()
	}

	if job.Attempts >= job.MaxAttempts {
		const burn = `UPDATE jobs SET status = ?, leased_until = NULL, last_error = ?, updated_at = ? WHERE id = ?`
		_, err := q.p.ExecCtx(ctx, burn, string(StatusFailed), errText, now, id)
		return err
	}

	delay := backoff(job.Attempts)
	const requeue = `UPDATE jobs
	                    SET status       = ?,
	                        leased_until = NULL,
	                        last_error   = ?,
	                        available_at = ?,
	                        updated_at   = ?
	                  WHERE id = ?`
	_, err = q.p.ExecCtx(ctx, requeue,
		string(StatusQueued), errText, now.Add(delay), now, id,
	)
	return err
}

// loadByID returns the row or (nil, nil) if not found.
func (q *Queue) loadByID(ctx context.Context, id string) (*Job, error) {
	const sel = `SELECT id, kind, payload, status, attempts, max_attempts,
	                    last_error, available_at, leased_until
	               FROM jobs WHERE id = ?`

	row := q.p.QueryRowCtx(ctx, sel, id)
	var (
		j       Job
		payload string
		status  string
		lastErr sql.NullString
		leased  sql.NullTime
	)
	err := row.Scan(&j.ID, &j.Kind, &payload, &status, &j.Attempts, &j.MaxAttempts,
		&lastErr, &j.AvailableAt, &leased)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j.Payload = []byte(payload)
	j.Status = Status(status)
	if lastErr.Valid {
		j.LastError = lastErr.String
	}
	if leased.Valid {
		t := leased.Time
		j.LeasedUntil = &t
	}
	return &j, nil
}

// backoff returns the delay for retry n (1-indexed). 2^n seconds, capped
// at 5 minutes — short enough that transient API blips recover quickly
// without burning CPU.
func backoff(attempt int) time.Duration {
	const cap = 5 * time.Minute
	secs := math.Pow(2, float64(attempt))
	d := time.Duration(secs) * time.Second
	if d > cap {
		return cap
	}
	return d
}
