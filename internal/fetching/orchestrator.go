package fetching

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/mileschou/devpulse/internal/pullrequest"
	"github.com/mileschou/devpulse/internal/repo"
)

const tracerName = "github.com/mileschou/devpulse/internal/fetching"

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

// FetchAllBuilds pulls all CI builds for one repo, upserts them, and
// back-fills GitHub commit-author logins for any rows that still lack one.
// Safe to re-run: upserts dedupe writes.
func (o *Orchestrator) FetchAllBuilds(ctx context.Context, r repo.Repo) (int, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.FetchAllBuilds",
		trace.WithAttributes(attribute.String("repo", r.Name.String())))
	defer span.End()

	builds, err := o.ci.ListAllBuilds(ctx, r.Name)
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

// FetchAllPullRequestsWithEnrichment pages through the full PR history,
// upserting and enriching each page as it arrives. Returns the total
// upserted count. Enrichment failures are logged but do not abort.
func (o *Orchestrator) FetchAllPullRequestsWithEnrichment(ctx context.Context, r repo.Repo) (int, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.FetchAllPullRequestsWithEnrichment",
		trace.WithAttributes(attribute.String("repo", r.Name.String())))
	defer span.End()

	var totalWritten int

	err := o.vcs.ListAllPullRequestsPageFunc(ctx, r.ID, r.Name, func(page []pullrequest.PullRequest) error {
		written, err := o.prs.UpsertMany(ctx, page)
		if err != nil {
			return err
		}
		totalWritten += written

		// UpsertMany writes back the canonical DB id on each entry, so
		// enrichment can run directly against the slice — one DB hit per
		// page instead of one FindByNumber per PR.
		for i := range page {
			if err := o.enrichOnePullRequest(ctx, r, page[i]); err != nil {
				o.logger.Warn("enrich pr failed",
					slog.String("repo", r.Name.String()),
					slog.Int("number", page[i].Number),
					slog.String("err", err.Error()),
				)
			}
		}
		return nil
	})

	if err != nil {
		span.RecordError(err)
		return totalWritten, err
	}
	return totalWritten, nil
}

// EnrichOnePullRequestByNumber loads a stored PR by number, fetches detail
// and reviews, and writes the enrichment patch. Returns (found, err).
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

	pr, err := o.prs.FindByNumber(ctx, r.ID, number)
	if err != nil {
		span.RecordError(err)
		return false, err
	}

	if err := o.enrichOnePullRequest(ctx, r, pr); err != nil {
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

// enrichOnePullRequest fetches PR detail + reviews and writes the
// derived lead-time fields. Reviews submitted before pr.ReadyAt are
// dropped so draft-period activity does not contribute to Pickup Time.
func (o *Orchestrator) enrichOnePullRequest(
	ctx context.Context,
	r repo.Repo,
	pr pullrequest.PullRequest,
) error {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.enrichOnePullRequest",
		trace.WithAttributes(attribute.Int("number", pr.Number)))
	defer span.End()

	detail, err := o.vcs.GetPullRequest(ctx, r.ID, r.Name, pr.Number)
	if err != nil {
		return fmt.Errorf("get detail: %w", err)
	}

	reviews, err := o.vcs.ListReviews(ctx, r.Name, pr.Number)
	if err != nil {
		return fmt.Errorf("list reviews: %w", err)
	}

	for _, rev := range reviews {
		if pr.ReadyAt != nil && rev.SubmittedAt.Before(*pr.ReadyAt) {
			continue
		}
		if err := o.reviews.Upsert(ctx, pr.ID, rev); err != nil {
			return fmt.Errorf("upsert review: %w", err)
		}
	}

	agg := pullrequest.AggregateReviews(reviews, pr.ReadyAt)
	patch := pullrequest.BuildEnrichmentPatch(pr, detail.Additions, detail.Deletions, agg)

	if err := o.prs.UpdateEnrichment(ctx, pr.ID, patch); err != nil {
		return fmt.Errorf("update enrichment: %w", err)
	}
	return nil
}
