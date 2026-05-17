<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Fetching\FetchOrchestrator;
use App\Models\Repo;
use RuntimeException;

/**
 * Enriches a single PR with detailed stats (additions/deletions) and review data,
 * writing results to dp_pull_requests and dp_pull_request_reviews.
 */
class EnrichPullRequestJob extends GitHubJob
{
    public function __construct(
        public readonly string $repoId,
        public readonly int $prNumber,
    ) {
    }

    public function handle(FetchOrchestrator $orchestrator): void
    {
        $repo = Repo::query()->findOrFail($this->repoId);

        $found = $orchestrator->enrichOnePullRequestByNumber($repo, $this->prNumber);
        if (!$found) {
            throw new RuntimeException("PR #{$this->prNumber} not found in repo `{$repo->name}`");
        }
    }
}
