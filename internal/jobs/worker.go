package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
)

const tracerName = "github.com/mileschou/devpulse/internal/jobs"

// HandlerFunc executes one job. The handler receives the job payload
// already decoded into raw bytes; concrete handlers decode into typed
// args. Returning an error triggers retry/burn-out via Queue.MarkFailed.
type HandlerFunc func(ctx context.Context, payload []byte) error

// Worker polls a Queue, dispatching to a per-kind HandlerFunc.
type Worker struct {
	q         *Queue
	handlers  map[string]HandlerFunc
	pollEvery time.Duration
	leaseFor  time.Duration
	logger    *slog.Logger
}

// NewWorker creates a Worker. handlers maps job.Kind -> HandlerFunc.
// pollEvery defaults to 5s; leaseFor defaults to 60s.
func NewWorker(q *Queue, handlers map[string]HandlerFunc, pollEvery, leaseFor time.Duration, logger *slog.Logger) *Worker {
	if pollEvery == 0 {
		pollEvery = 5 * time.Second
	}
	if leaseFor == 0 {
		leaseFor = 60 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{
		q: q, handlers: handlers,
		pollEvery: pollEvery, leaseFor: leaseFor,
		logger: logger,
	}
}

// Run blocks until ctx is done. For each tick it tries every registered
// kind once. Returns ctx.Err() on shutdown.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.pollEvery)
	defer ticker.Stop()

	for {
		if err := w.tick(ctx); err != nil {
			w.logger.Warn("job worker tick error", slog.String("err", err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// tick runs at most one job per registered kind. We do not pre-drain
// all available jobs because we want fair-share progress across kinds.
func (w *Worker) tick(ctx context.Context) error {
	for kind, handler := range w.handlers {
		job, err := w.q.Lease(ctx, kind, w.leaseFor)
		if err != nil {
			return err
		}
		if job == nil {
			continue
		}

		w.process(ctx, job, handler)
	}
	return nil
}

func (w *Worker) process(ctx context.Context, job *Job, handler HandlerFunc) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "job."+job.Kind,
		trace.WithAttributes(
			attribute.String("job.id", job.ID),
			attribute.Int("job.attempts", job.Attempts),
		))
	defer span.End()

	if err := handler(ctx, job.Payload); err != nil {
		span.RecordError(err)
		if mfErr := w.q.MarkFailed(ctx, job.ID, err); mfErr != nil {
			w.logger.Error("mark failed",
				slog.String("job.id", job.ID), slog.String("err", mfErr.Error()))
		}
		return
	}
	if err := w.q.MarkDone(ctx, job.ID); err != nil {
		w.logger.Error("mark done", slog.String("job.id", job.ID), slog.String("err", err.Error()))
	}
}

// ----- concrete job kinds -----

const KindEnrichPullRequest = "enrich_pull_request"

// EnrichPullRequestArgs is the payload for an enrich_pull_request job.
type EnrichPullRequestArgs struct {
	RepoID string `json:"repo_id"`
	Number int    `json:"number"`
}

// NewEnrichPullRequestHandler returns a HandlerFunc that calls the
// orchestrator's per-PR enrichment path. The handler fetches the Repo
// row by ID via repos so callers don't have to embed the full repo in
// the payload.
func NewEnrichPullRequestHandler(
	orch *fetching.Orchestrator,
	repos *persistence.RepoPersister,
) HandlerFunc {
	return func(ctx context.Context, payload []byte) error {
		var args EnrichPullRequestArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return fmt.Errorf("enrich-pr: bad payload: %w", err)
		}
		r, err := repos.FindByID(ctx, args.RepoID)
		if err != nil {
			return fmt.Errorf("enrich-pr: load repo: %w", err)
		}
		found, err := orch.EnrichOnePullRequestByNumber(ctx, r, args.Number)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("enrich-pr: PR #%d not found", args.Number)
		}
		return nil
	}
}

// Compile-time check that the repo persister actually exposes FindByID.
var _ interface {
	FindByID(ctx context.Context, id string) (repo.Repo, error)
} = (*persistence.RepoPersister)(nil)
