package fetching

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
)

const tracerName = "github.com/mileschou/devpulse/internal/fetching"

// buildRetryOverlap is how far behind the DB watermark we keep looking
// when paging Travis. Travis allows a build to be retried — the new
// build gets a fresh id but its started_at may sit slightly before the
// previous max we have on file. Pulling a small overlap window covers
// those without giving up the incremental property. The retry build
// itself has a new external_id, so the (repo_id, external_id) unique
// constraint silently dedupes everything that was already stored.
//
// Five minutes was picked to cover routine retry latency (typically
// sub-minute) with comfortable headroom; bump it later if a real-world
// retry is observed to land further out.
const buildRetryOverlap = 5 * time.Minute

// Orchestrator coordinates CI + VCS fetch and enrichment for one repo.
type Orchestrator struct {
	ci      CIProvider
	vcs     VCSProvider
	builds  BuildWriter
	prs     PullRequestWriter
	reviews ReviewWriter
	logger  *slog.Logger
}

// NewOrchestrator wires the dependencies.
func NewOrchestrator(
	ci CIProvider,
	vcs VCSProvider,
	builds BuildWriter,
	prs PullRequestWriter,
	reviews ReviewWriter,
	logger *slog.Logger,
) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Orchestrator{
		ci: ci, vcs: vcs, builds: builds, prs: prs, reviews: reviews, logger: logger,
	}
}

// FetchAllBuilds pulls CI builds for one repo and upserts them, then
// back-fills GitHub commit-author logins for any rows that still lack
// one.
//
// Incremental sync: the fetch is bounded by a DB-derived watermark.
// On every call we read MAX(started_at) for the repo, subtract a small
// retry overlap, and pass that as the `since` cursor to
// CIProvider.ListBuildsSince. Travis is paged in id-desc order and
// stops once a page reaches the watermark, so a routine sync on a
// quiet repo costs ~one page even if the full history has 10k+
// builds. The retry overlap window covers Travis "retry build"
// objects whose started_at lands slightly behind the previous max.
//
// First run: when the DB has no builds for the repo, MaxStartedAt
// returns (_, false, _) and we pass a zero time, which the provider
// treats as cold-start — walk the full upstream history (the only
// path that pays the linear page cost).
//
// Safe to re-run: upserts dedupe writes via (repo_id, external_id)
// unique, so the overlap window cannot create duplicates.
func (o *Orchestrator) FetchAllBuilds(ctx context.Context, r repo.Repo) (int, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.FetchAllBuilds",
		trace.WithAttributes(attribute.String("repo", r.Name.String())))
	defer span.End()

	watermark, hasWatermark, err := o.builds.MaxStartedAt(ctx, r.ID)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("get build watermark: %w", err)
	}

	var since time.Time
	if hasWatermark {
		since = watermark.Add(-buildRetryOverlap)
	}
	span.SetAttributes(attribute.Bool("watermark.has", hasWatermark))
	if hasWatermark {
		// Only emit the cursor when it is meaningful; on cold start the
		// zero time would render as 0001-01-01T00:00:00Z on traces and
		// look like a real value to anyone scanning the span.
		span.SetAttributes(attribute.String("watermark.since", since.Format(time.RFC3339)))
	}

	builds, err := o.ci.ListBuildsSince(ctx, r.Name, since)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	written, err := o.builds.UpsertMany(ctx, r.ID, builds)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	if err := o.enrichBuildAuthors(ctx, r); err != nil {
		o.logger.Warn("enrich build authors failed",
			slog.String("repo", r.Name.String()),
			slog.String("err", err.Error()),
		)
	}

	return written, nil
}

// BackfillPullRequestsByNumber syncs every PR number from
// max(repo.PRSyncStartNumber, MAX(number)+1) up to the upstream max,
// ascending. Each PR is atomic — detail fetch, upsert, reviews, and
// enrichment all complete before moving on — so MAX(number) in the DB
// always points to a fully synced PR. An interrupted run resumes
// naturally on the next call without any extra state.
//
// Error policy:
//   - Upstream 404 (issue/deleted) is skipped silently; the loop keeps
//     advancing. Long runs of 404s in a row do incur repeat probes on
//     subsequent runs because MAX(number) does not advance until a real
//     PR lands — accepted trade-off for not maintaining an absent-set.
//   - Any other error from a single PR aborts the whole repo's PR sync
//     immediately (fail-fast). DB state remains at the last successful
//     PR so the next run picks up there.
//   - ctx cancellation is honored between iterations so Ctrl-C stays
//     responsive even on large backfills.
//
// Returns the count of PRs successfully written this round (404 skips
// don't count). Used for the "Synced X pull requests: written=N" line.
func (o *Orchestrator) BackfillPullRequestsByNumber(ctx context.Context, r repo.Repo) (int, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.BackfillPullRequestsByNumber",
		trace.WithAttributes(attribute.String("repo", r.Name.String())))
	defer span.End()

	remoteMax, err := o.vcs.GetLatestPRNumber(ctx, r.Name)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("get latest pr number: %w", err)
	}
	if remoteMax == 0 {
		return 0, nil
	}

	dbMax, hasDBMax, err := o.prs.MaxNumber(ctx, r.ID)
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("get db max number: %w", err)
	}

	start := max(r.PRSyncStartNumber, 1)
	if hasDBMax {
		start = max(start, dbMax+1)
	}

	span.SetAttributes(
		attribute.Int("start", start),
		attribute.Int("remote_max", remoteMax),
	)

	var written int
	for n := start; n <= remoteMax; n++ {
		if err := ctx.Err(); err != nil {
			span.RecordError(err)
			return written, err
		}

		ok, err := o.syncOnePullRequestByNumber(ctx, r, n)
		if err != nil {
			span.RecordError(err)
			return written, fmt.Errorf("sync pr #%d: %w", n, err)
		}
		if ok {
			written++
		}
	}
	span.SetAttributes(attribute.Int("written", written))
	return written, nil
}

// syncOnePullRequestByNumber fetches PR #n end-to-end and writes it in
// a single upsert that already carries the computed enrichment fields.
// This is what makes the by-number backfill resumable from DB MAX
// without per-PR progress state: when the row exists, every column on
// it — including additions/deletions and lead-time metrics — was
// written in one atomic step.
//
// Returns (false, nil) when the number is a 404 (issue / deleted) so
// the caller can keep advancing without counting it.
//
// Failure modes:
//   - detail / reviews fetch failure → returns the wrapped error; no
//     DB writes happened. Next run probes the same number.
//   - PR upsert failure → returns the wrapped error; no DB writes
//     happened. Same recovery as above.
//   - Review-row upsert failure → the PR row was already committed
//     (with enrichment), so dbMax advances. Individual review-row
//     gaps can be repaired by `devpulse pr sync <n>`. This trade-off
//     is documented because making reviews tx-atomic with the PR row
//     would require plumbing transactions through the writer
//     interfaces — out of scope for this iteration.
func (o *Orchestrator) syncOnePullRequestByNumber(
	ctx context.Context,
	r repo.Repo,
	number int,
) (bool, error) {
	detail, err := o.vcs.GetPullRequest(ctx, r.ID, r.Name, number)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			o.logger.Debug("pr skipped (not a pr / deleted)",
				slog.String("repo", r.Name.String()),
				slog.Int("number", number),
			)
			return false, nil
		}
		return false, fmt.Errorf("get detail: %w", err)
	}

	reviews, err := o.vcs.ListReviews(ctx, r.Name, number)
	if err != nil {
		return false, fmt.Errorf("list reviews: %w", err)
	}

	// Compute enrichment BEFORE the PR row is written, so the upsert
	// can carry the derived columns in one shot. Reviews submitted
	// during the draft period are dropped from both the aggregation
	// and the per-row upsert below.
	//
	// TotalChangedLines is recomputed here even though the GitHub
	// provider's toDomain also sets it — this is the single point that
	// every PR write passes through, so the cheap a+b makes the
	// invariant `total == additions + deletions` independent of how
	// `detail` was constructed (e.g. an alternate VCSProvider or test
	// fake that doesn't go through toDomain).
	detail.TotalChangedLines = detail.Additions + detail.Deletions
	agg := pullrequest.AggregateReviews(reviews, detail.ReadyAt)
	detail.FirstReviewAt = agg.FirstReviewAt
	detail.FirstApprovedAt = agg.FirstApprovedAt
	detail.TimeToApproval = pullrequest.ComputeTimeToApproval(detail.ReadyAt, agg.FirstApprovedAt)
	detail.TimeToMerge = pullrequest.ComputeTimeToMerge(agg.FirstApprovedAt, detail.MergedAt)

	// Single-element slice is intentional: UpsertMany writes the
	// canonical id back into batch[0].ID for the review loop below.
	batch := []pullrequest.PullRequest{detail}
	if _, err := o.prs.UpsertMany(ctx, batch); err != nil {
		return false, fmt.Errorf("upsert: %w", err)
	}
	prID := batch[0].ID

	for _, rev := range reviews {
		if detail.ReadyAt != nil && rev.SubmittedAt.Before(*detail.ReadyAt) {
			continue
		}
		if err := o.reviews.Upsert(ctx, prID, rev); err != nil {
			return false, fmt.Errorf("upsert review: %w", err)
		}
	}
	return true, nil
}

// EnrichOnePullRequestByNumber re-syncs one PR that is already in the
// store. It is the `devpulse pr sync` entry point — its job is to
// refresh detail + reviews + enrichment for one number on demand,
// typically to repair a row whose backfill was interrupted mid-PR.
//
// Return contract:
//   - (true, nil):  PR refreshed successfully.
//   - (false, nil): PR is not in the local store. The caller decides how
//     to surface this (the CLI prints a "run `repo sync` first" hint).
//     This sentinel-free signal lets the CLI distinguish "user typo"
//     from a real failure without depending on persistence internals.
//   - (true, err):  PR existed locally but the refresh failed (network,
//     upstream 404, write error). Upstream 404 is surfaced as an error
//     here — unlike the backfill loop which skips 404s — because an
//     operator-issued `pr sync <n>` against a non-existent number is
//     most likely a typo and should not be swallowed.
func (o *Orchestrator) EnrichOnePullRequestByNumber(
	ctx context.Context,
	r repo.Repo,
	number int,
) (bool, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.EnrichOnePullRequestByNumber",
		trace.WithAttributes(
			attribute.String("repo", r.Name.String()),
			attribute.Int("number", number),
		))
	defer span.End()

	if _, err := o.prs.FindByNumber(ctx, r.ID, number); err != nil {
		if errors.Is(err, persistence.ErrPullRequestNotFound) {
			return false, nil
		}
		span.RecordError(err)
		return false, err
	}

	ok, err := o.syncOnePullRequestByNumber(ctx, r, number)
	if err != nil {
		span.RecordError(err)
		return true, err
	}
	if !ok {
		err := fmt.Errorf("pr #%d gone upstream", number)
		span.RecordError(err)
		return true, err
	}
	return true, nil
}

// enrichBuildAuthors queries the DB for build rows whose author_account
// is still NULL and back-fills them in one bulk GitHub lookup. Driving
// the query from DB state (rather than the just-fetched slice) keeps
// re-runs O(missing) instead of O(all fetched).
func (o *Orchestrator) enrichBuildAuthors(ctx context.Context, r repo.Repo) error {
	shas, err := o.builds.ListMissingAuthorSHAs(ctx, r.ID)
	if err != nil {
		return fmt.Errorf("list missing author shas: %w", err)
	}
	if len(shas) == 0 {
		return nil
	}

	logins, err := o.vcs.GetCommitAuthorAccountsBulk(ctx, r.Name, shas)
	if err != nil {
		return fmt.Errorf("bulk author lookup: %w", err)
	}

	for sha, login := range logins {
		if login == nil {
			continue
		}
		if err := o.builds.UpdateAuthorBySHA(ctx, r.ID, sha, *login); err != nil {
			o.logger.Warn("update author failed",
				slog.String("sha", sha.String()),
				slog.String("err", err.Error()),
			)
		}
	}
	return nil
}
