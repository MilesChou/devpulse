<?php

declare(strict_types=1);

namespace App\Fetching;

use App\Domain\Ci\CiProvider;
use App\Domain\Shared\MonthRange;
use App\Domain\Shared\RepoFullName;
use App\Infrastructure\Vcs\GitHub\GitHubProvider;
use App\Models\Group;
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
        $repoFullName = new RepoFullName($repo->full_name);

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
        $this->cache->markComplete($repo->id, Dataset::Builds, $monthLabel);

        return $written;
    }

    private function fetchPullRequests(
        Repo $repo,
        RepoFullName $repoFullName,
        MonthRange $month,
        string $monthLabel,
    ): int {
        $pulls = $this->vcsProvider->listPullRequestsInMonth($repoFullName, $month);
        $written = $this->pullRequestRepository->upsertMany($repo->id, $pulls);
        $this->cache->markComplete($repo->id, Dataset::PullRequests, $monthLabel);

        return $written;
    }
}
