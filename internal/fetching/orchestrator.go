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

// Orchestrator coordinates CI + VCS fetch and enrichment for one repo/month.
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

// FetchBuilds pulls CI builds for one repo / month, upserts them, and
// back-fills GitHub commit-author logins for any rows that still lack one.
// Safe to re-run for the same month: upserts dedupe writes.
func (o *Orchestrator) FetchBuilds(ctx context.Context, r repo.Repo, month MonthRange) BuildsFetchOutcome {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.FetchBuilds",
		trace.WithAttributes(
			attribute.String("repo", r.Name.String()),
			attribute.String("month", month.String()),
		))
	defer span.End()

	outcome := BuildsFetchOutcome{RepoFullName: r.Name.String()}

	written, err := o.fetchBuilds(ctx, r, month)
	if err != nil {
		outcome.Error = fmt.Errorf("fetch builds: %w", err)
		span.RecordError(err)
		return outcome
	}
	outcome.BuildsWritten = written
	return outcome
}

// FetchPullRequests pulls PRs (and their reviews / enrichment) for one
// repo / month. Safe to re-run.
func (o *Orchestrator) FetchPullRequests(ctx context.Context, r repo.Repo, month MonthRange) PullRequestsFetchOutcome {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.FetchPullRequests",
		trace.WithAttributes(
			attribute.String("repo", r.Name.String()),
			attribute.String("month", month.String()),
		))
	defer span.End()

	outcome := PullRequestsFetchOutcome{RepoFullName: r.Name.String()}

	written, err := o.fetchPullRequests(ctx, r, month)
	if err != nil {
		outcome.Error = fmt.Errorf("fetch pull requests: %w", err)
		span.RecordError(err)
		return outcome
	}
	outcome.PullRequestsWritten = written
	return outcome
}

// FetchAllPullRequests pulls every historical PR for the repo and upserts.
// Returned int is the count actually written.
func (o *Orchestrator) FetchAllPullRequests(ctx context.Context, r repo.Repo) (int, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.FetchAllPullRequests",
		trace.WithAttributes(attribute.String("repo", r.Name.String())))
	defer span.End()

	pulls, err := o.vcs.ListAllPullRequests(ctx, r.ID, r.Name)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}
	return o.prs.UpsertMany(ctx, pulls)
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

		for _, pr := range page {
			stored, err := o.prs.FindByNumber(ctx, r.ID, pr.Number)
			if err != nil {
				o.logger.Warn("find pr for enrich failed",
					slog.String("repo", r.Name.String()),
					slog.Int("number", pr.Number),
					slog.String("err", err.Error()),
				)
				continue
			}
			if err := o.enrichOnePullRequest(ctx, r, stored); err != nil {
				o.logger.Warn("enrich pr failed",
					slog.String("repo", r.Name.String()),
					slog.Int("number", pr.Number),
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

// fetchBuilds pulls Travis builds for the month, upserts them, then back-
// fills GitHub commit-author logins for any rows that still lack one.
func (o *Orchestrator) fetchBuilds(ctx context.Context, r repo.Repo, month MonthRange) (int, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.fetchBuilds")
	defer span.End()

	builds, err := o.ci.ListBuildsInMonth(ctx, r.Name, month)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	written, err := o.builds.UpsertMany(ctx, r.ID, builds)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	if err := o.enrichBuildAuthors(ctx, r, month); err != nil {
		// Author enrichment failure should not invalidate the build write;
		// log it and let the next month's fetch retry the same SHAs.
		o.logger.Warn("enrich build authors failed",
			slog.String("repo", r.Name.String()),
			slog.String("err", err.Error()),
		)
	}

	return written, nil
}

func (o *Orchestrator) enrichBuildAuthors(ctx context.Context, r repo.Repo, month MonthRange) error {
	shas, err := o.builds.ListMissingAuthorSHAs(ctx, r.ID, month)
	if err != nil {
		return fmt.Errorf("list missing shas: %w", err)
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

func (o *Orchestrator) fetchPullRequests(ctx context.Context, r repo.Repo, month MonthRange) (int, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Orchestrator.fetchPullRequests")
	defer span.End()

	pulls, err := o.vcs.ListPullRequestsInMonth(ctx, r.ID, r.Name, month)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	written, err := o.prs.UpsertMany(ctx, pulls)
	if err != nil {
		span.RecordError(err)
		return 0, err
	}

	// Enrich each PR in the month sequentially. Failures are logged but do
	// not abort the rest — the orchestrator records as much accurate data
	// as it can, and a retry on the next run picks up the remainder.
	stored, err := o.prs.ListInMonth(ctx, r.ID, month)
	if err != nil {
		return written, err
	}
	for _, pr := range stored {
		if err := o.enrichOnePullRequest(ctx, r, pr); err != nil {
			o.logger.Warn("enrich pr failed",
				slog.String("repo", r.Name.String()),
				slog.Int("number", pr.Number),
				slog.String("err", err.Error()),
			)
		}
	}
	return written, nil
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
