<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Fetching\FetchOrchestrator;
use App\Infrastructure\Vcs\GitHub\GitHubProvider;
use App\Models\Repo;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\Middleware\RateLimited;
use Illuminate\Queue\SerializesModels;
use RuntimeException;

/**
 * Enriches a single PR with detailed stats (additions/deletions) and review data,
 * writing results to dp_pull_requests and dp_pull_request_reviews.
 */
class EnrichPullRequestJob implements ShouldQueue
{
    use Dispatchable;
    use InteractsWithQueue;
    use Queueable;
    use SerializesModels;

    public int $tries = 3;

    public int $timeout = 600;

    public function __construct(
        public readonly string $repoId,
        public readonly int $prNumber,
    ) {
    }

    /**
     * @return list<object>
     */
    public function middleware(): array
    {
        return [
            new RateLimited(GitHubProvider::RATE_LIMITER),
        ];
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
