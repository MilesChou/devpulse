// Package syncrun runs one repo sync at a time in the background, for
// `devpulse serve`: the API triggers a sync and returns immediately, and
// clients poll Status. Syncs are serialized because GitHub and CI
// providers rate-limit per token, the same reason `devpulse sync` walks
// repos sequentially.
package syncrun

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/mileschou/devpulse/internal/repo"
)

// Func syncs one repo end to end (the `devpulse repo sync` flow).
type Func func(ctx context.Context, r repo.Repo) error

// ErrBusy is returned by Start while another sync is running.
var ErrBusy = errors.New("syncrun: a sync is already running")

// Status is a snapshot of the runner.
type Status struct {
	// Running is the repo being synced, or empty when idle.
	Running   string     `json:"running"`
	StartedAt *time.Time `json:"started_at"`

	// Last* describe the most recent finished sync, if any.
	LastRepo       string     `json:"last_repo"`
	LastFinishedAt *time.Time `json:"last_finished_at"`
	LastError      string     `json:"last_error"`
}

// Runner serializes background syncs. Syncs run under the context given
// to New, so shutting the server down cancels a sync in progress; the
// next sync resumes from database state.
type Runner struct {
	ctx    context.Context
	fn     Func
	logger *slog.Logger
	now    func() time.Time

	mu     sync.Mutex
	status Status
	done   chan struct{} // closed when the current sync ends
}

func New(ctx context.Context, fn Func, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{ctx: ctx, fn: fn, logger: logger, now: time.Now}
}

// Start begins syncing r in the background, or returns ErrBusy.
func (rn *Runner) Start(r repo.Repo) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if rn.status.Running != "" {
		return ErrBusy
	}
	started := rn.now().UTC()
	rn.status.Running = r.Name.String()
	rn.status.StartedAt = &started
	done := make(chan struct{})
	rn.done = done

	go func() {
		defer close(done)
		rn.logger.Info("sync started", slog.String("repo", r.Name.String()))
		err := rn.fn(rn.ctx, r)

		rn.mu.Lock()
		defer rn.mu.Unlock()
		finished := rn.now().UTC()
		rn.status = Status{
			LastRepo:       r.Name.String(),
			LastFinishedAt: &finished,
		}
		if err != nil {
			rn.status.LastError = err.Error()
			rn.logger.Warn("sync failed", slog.String("repo", r.Name.String()), slog.String("err", err.Error()))
			return
		}
		rn.logger.Info("sync finished", slog.String("repo", r.Name.String()))
	}()
	return nil
}

// Status returns a snapshot.
func (rn *Runner) Status() Status {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.status
}

// Wait blocks until the current sync, if any, ends. For tests and
// graceful shutdown.
func (rn *Runner) Wait() {
	rn.mu.Lock()
	done := rn.done
	rn.mu.Unlock()
	if done != nil {
		<-done
	}
}
