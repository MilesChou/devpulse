<?php

declare(strict_types=1);

namespace App\Fetching;

use DevPulse\Ci\CiProvider;
use DevPulse\Shared\MonthRange;
use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\ReviewState;
use App\Infrastructure\Vcs\GitHub\GitHubProvider;
use App\Models\Build;
use App\Models\PullRequest;
use App\Models\PullRequestReview;
use App\Models\Repo;
use App\Persistence\Repository\BuildRepository;
use App\Persistence\Repository\PullRequestRepository;
use Throwable;

final class FetchOrchestrator
{
    public function __construct(
        private readonly CiProvider $ciProvider,
        private readonly GitHubProvider $vcsProvider,
        private readonly BuildRepository $buildRepository,
        private readonly PullRequestRepository $pullRequestRepository,
    ) {
    }

    /**
     * Safe to re-run for the same month; upsert handles duplicates.
     */
    public function fetch(Repo $repo, MonthRange $month): RepoFetchOutcome
    {
        $repoFullName = new RepoFullName($repo->name);

        try {
            $buildsWritten = $this->fetchBuilds($repo, $repoFullName, $month);
            $prsWritten = $this->fetchPullRequests($repo, $repoFullName, $month);
        } catch (Throwable $e) {
            return new RepoFetchOutcome(
                repoFullName: (string)$repoFullName,
                buildsWritten: 0,
                pullRequestsWritten: 0,
                error: $e->getMessage(),
            );
        }

        return new RepoFetchOutcome(
            repoFullName: (string)$repoFullName,
            buildsWritten: $buildsWritten,
            pullRequestsWritten: $prsWritten,
        );
    }

    private function fetchBuilds(Repo $repo, RepoFullName $repoFullName, MonthRange $month): int
    {
        $builds = $this->ciProvider->listBuildsInMonth($repoFullName, $month);
        $written = $this->buildRepository->upsertMany($repo->id, $builds);
        $this->enrichBuildAuthors($repo, $repoFullName, $month);

        return $written;
    }

    /**
     * Back-fill author_account for builds written this month using a bulk GitHub commit-author query.
     *
     * Travis payloads do not include a GitHub login, so a second lookup is required.
     */
    private function enrichBuildAuthors(Repo $repo, RepoFullName $repoFullName, MonthRange $month): void
    {
        /** @var list<string> $shas */
        $shas = Build::query()
            ->where('repo_id', $repo->id)
            ->where('started_at', '>=', $month->start)
            ->where('started_at', '<', $month->end)
            ->whereNull('author_account')
            ->pluck('commit_sha')
            ->unique()
            ->values()
            ->all();

        if ($shas === []) {
            return;
        }

        $sha2login = $this->vcsProvider->getCommitAuthorAccountsBulk($repoFullName, $shas);

        foreach ($sha2login as $sha => $login) {
            if ($login === null) {
                continue;
            }
            Build::query()
                ->where('repo_id', $repo->id)
                ->where('commit_sha', $sha)
                ->update(['author_account' => $login]);
        }
    }

    private function fetchPullRequests(Repo $repo, RepoFullName $repoFullName, MonthRange $month): int
    {
        $pulls = $this->vcsProvider->listPullRequestsInMonth($repo->id, $repoFullName, $month);
        $written = $this->pullRequestRepository->upsertMany($pulls);
        $this->enrichPullRequestReviews($repo, $repoFullName, $month);

        return $written;
    }

    /**
     * Fetch all historical PRs for the given repo (state=all, no month filter) and upsert the list. Returns the number of rows written.
     */
    public function fetchAllPullRequests(Repo $repo): int
    {
        $pulls = $this->vcsProvider->listAllPullRequests($repo->id, new RepoFullName($repo->name));

        return $this->pullRequestRepository->upsertMany($pulls);
    }

    public function enrichOnePullRequestByNumber(Repo $repo, int $prNumber): bool
    {
        $pr = PullRequest::query()
            ->where('repo_id', $repo->id)
            ->where('number', $prNumber)
            ->first(['id', 'number', 'ready_at', 'merged_at']);

        if ($pr === null) {
            return false;
        }

        $repoFullName = new RepoFullName($repo->name);
        $this->enrichOnePullRequest($pr, $repoFullName);

        return true;
    }

    private function enrichPullRequestReviews(Repo $repo, RepoFullName $repoFullName, MonthRange $month): void
    {
        $prs = PullRequest::query()
            ->where('repo_id', $repo->id)
            ->where('pr_created_at', '>=', $month->start)
            ->where('pr_created_at', '<', $month->end)
            ->get(['id', 'repo_id', 'number', 'ready_at', 'merged_at']);

        foreach ($prs as $pr) {
            $this->enrichOnePullRequest($pr, $repoFullName);
        }
    }

    private function enrichOnePullRequest(PullRequest $pr, RepoFullName $repoFullName): void
    {
        $detail = $this->vcsProvider->getPullRequest($pr->repo_id, $repoFullName, $pr->number);
        $reviews = $this->vcsProvider->listReviews($repoFullName, $pr->number);

        $firstReviewAt = null;
        $firstApprovedAt = null;

        foreach ($reviews as $review) {
            // Ignore reviews submitted before ready_at; draft-period reviews do not count.
            if ($pr->ready_at !== null && $review->submittedAt < $pr->ready_at) {
                continue;
            }

            PullRequestReview::updateOrCreate(
                [
                    'pull_request_id' => $pr->id,
                    'reviewer_account' => $review->reviewerAccount,
                    'submitted_at' => $review->submittedAt,
                ],
                ['state' => $review->state->value],
            );

            if ($firstReviewAt === null || $review->submittedAt < $firstReviewAt) {
                $firstReviewAt = $review->submittedAt;
            }

            if ($review->state === ReviewState::Approved) {
                if ($firstApprovedAt === null || $review->submittedAt < $firstApprovedAt) {
                    $firstApprovedAt = $review->submittedAt;
                }
            }
        }

        $timeToApproval = ($pr->ready_at !== null && $firstApprovedAt !== null)
            ? $firstApprovedAt->getTimestamp() - $pr->ready_at->getTimestamp()
            : null;

        $timeToMerge = ($firstApprovedAt !== null && $pr->merged_at !== null)
            ? $pr->merged_at->getTimestamp() - $firstApprovedAt->getTimestamp()
            : null;

        $pr->update([
            'additions' => $detail->changes()->additions,
            'deletions' => $detail->changes()->deletions,
            'total_changed_lines' => $detail->changes()->total(),
            'first_review_at' => $firstReviewAt,
            'first_approved_at' => $firstApprovedAt,
            'time_to_approval' => $timeToApproval,
            'time_to_merge' => $timeToMerge,
        ]);
    }
}
