<?php

declare(strict_types=1);

namespace App\Fetching;

use App\Aggregation\PrSizeBucket;
use DevPulse\Ci\CiProvider;
use DevPulse\Shared\MonthRange;
use DevPulse\Shared\RepoFullName;
use App\Infrastructure\Vcs\GitHub\GitHubProvider;
use DevPulse\Vcs\Filter\BotFilter;
use App\Models\Build;
use App\Models\Group;
use App\Models\PullRequest;
use App\Models\PullRequestReview;
use App\Models\Repo;
use App\Persistence\Enum\Dataset;
use App\Persistence\MonthFetchCache;
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
        private readonly MonthFetchCache $cache,
        private readonly BotFilter $botFilter,
        private readonly PrSizeBucket $sizeBucket,
    ) {
    }

    /**
     * 對 group 中每個 repo 撈該月的 builds + PRs。
     *
     * cache 規則：對 (repo_id, dataset, month) 已 complete 的組合直接 skip，除非 force=true。
     * 寫入採 upsert，重跑同月安全（不會重複插入）。
     */
    public function fetch(Group $group, MonthRange $month, bool $force = false): FetchResult
    {
        $monthLabel = $month->start->format('Y-m');
        $outcomes = [];

        foreach ($group->repos()->get() as $repo) {
            $outcomes[$repo->id] = $this->fetchRepo($repo, $month, $monthLabel, $force);
        }

        return new FetchResult(month: $monthLabel, repos: $outcomes);
    }

    private function fetchRepo(Repo $repo, MonthRange $month, string $monthLabel, bool $force): RepoFetchOutcome
    {
        $repoFullName = new RepoFullName($repo->name);

        $shouldFetchBuilds = $force || $this->cache->shouldFetch($repo->id, Dataset::Builds, $monthLabel);
        $shouldFetchPrs = $force || $this->cache->shouldFetch($repo->id, Dataset::PullRequests, $monthLabel);

        if (!$shouldFetchBuilds && !$shouldFetchPrs) {
            return new RepoFetchOutcome(
                repoFullName: (string)$repoFullName,
                skipped: true,
                buildsWritten: 0,
                pullRequestsWritten: 0,
            );
        }

        try {
            $buildsWritten = $shouldFetchBuilds ? $this->fetchBuilds($repo, $repoFullName, $month, $monthLabel) : 0;
            $prsWritten = $shouldFetchPrs ? $this->fetchPullRequests($repo, $repoFullName, $month, $monthLabel) : 0;
        } catch (Throwable $e) {
            return new RepoFetchOutcome(
                repoFullName: (string)$repoFullName,
                skipped: false,
                buildsWritten: 0,
                pullRequestsWritten: 0,
                error: $e->getMessage(),
            );
        }

        return new RepoFetchOutcome(
            repoFullName: (string)$repoFullName,
            skipped: false,
            buildsWritten: $buildsWritten,
            pullRequestsWritten: $prsWritten,
        );
    }

    private function fetchBuilds(Repo $repo, RepoFullName $repoFullName, MonthRange $month, string $monthLabel): int
    {
        $builds = $this->ciProvider->listBuildsInMonth($repoFullName, $month);
        $written = $this->buildRepository->upsertMany($repo->id, $builds);
        $this->enrichBuildAuthors($repo, $repoFullName, $month);
        $this->cache->markComplete($repo->id, Dataset::Builds, $monthLabel);

        return $written;
    }

    /**
     * 把該月剛寫進 DB 的 builds 的 author_account 用 GitHub commit author bulk query 補齊。
     *
     * Travis payload 不含 GitHub login，必須二次撈才能對應到 member。
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

    private function fetchPullRequests(
        Repo $repo,
        RepoFullName $repoFullName,
        MonthRange $month,
        string $monthLabel,
    ): int {
        $pulls = $this->vcsProvider->listPullRequestsInMonth($repo->id, $repoFullName, $month);
        // 過濾 bot 開的 PR（spec 4.6）；list endpoint 撈下來、寫入前先濾掉
        $filtered = (function () use ($pulls): \Generator {
            foreach ($pulls as $pr) {
                if ($this->botFilter->isBotPullRequest($pr)) {
                    continue;
                }
                yield $pr;
            }
        })();
        $written = $this->pullRequestRepository->upsertMany($filtered);
        $this->enrichPullRequestReviews($repo, $repoFullName, $month);
        $this->cache->markComplete($repo->id, Dataset::PullRequests, $monthLabel);

        return $written;
    }

    /**
     * 對該月 PR 補 detail（拿到 additions/deletions 算 size bucket）+ first_review_at。
     *
     * GitHub list endpoint 不回 additions/deletions，必須對每個 PR 打 detail；
     * reviews 也要另外撈 GraphQL。每筆 PR 兩次 API 請求，量大時可考慮 GraphQL 一次抓。
     */
    /**
     * 對單一 PR 強制重跑 enrich（忽略 size_bucket 狀態）。
     */
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
            ->whereNull('size_bucket')
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
            if ($this->botFilter->isBotReview($review)) {
                continue;
            }

            // draft 期間的 review 不計入（ready_at 之後才算）
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

            if ($review->state === \DevPulse\Vcs\ReviewState::Approved) {
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

        $totalLines = $detail->changes()->total();
        $pr->update([
            'additions' => $detail->changes()->additions,
            'deletions' => $detail->changes()->deletions,
            'total_changed_lines' => $totalLines,
            'size_bucket' => $this->sizeBucket->classify($totalLines),
            'first_review_at' => $firstReviewAt,
            'first_approved_at' => $firstApprovedAt,
            'time_to_approval' => $timeToApproval,
            'time_to_merge' => $timeToMerge,
        ]);
    }
}
