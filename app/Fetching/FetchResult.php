<?php

declare(strict_types=1);

namespace App\Fetching;

final readonly class FetchResult
{
    /**
     * @param array<string, RepoFetchOutcome> $repos repo 的個別結果（key 為 repo_id ULID）
     */
    public function __construct(
        public string $month,
        public array $repos,
    ) {
    }

    public function totalBuildsWritten(): int
    {
        return array_sum(array_map(fn (RepoFetchOutcome $r) => $r->buildsWritten, $this->repos));
    }

    public function totalPullRequestsWritten(): int
    {
        return array_sum(array_map(fn (RepoFetchOutcome $r) => $r->pullRequestsWritten, $this->repos));
    }

    public function totalReposSkipped(): int
    {
        return count(array_filter($this->repos, fn (RepoFetchOutcome $r) => $r->skipped));
    }
}
