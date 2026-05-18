<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Fetching\FetchOrchestrator;
use App\Infrastructure\Vcs\GitHub\GitHubProvider;
use App\Models\PullRequest;
use App\Models\Repo;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\Middleware\RateLimited;
use Illuminate\Queue\SerializesModels;

/**
 * Fetches the full PR history for a repo (state=all) and upserts the list,
 * then dispatches one EnrichPullRequestJob per unenriched PR.
 */
class FetchAllPullRequestsJob implements ShouldQueue
{
    use Dispatchable;
    use InteractsWithQueue;
    use Queueable;
    use SerializesModels;

    public int $tries = 3;

    public int $timeout = 600;

    public function __construct(public readonly string $repoId)
    {
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
        $orchestrator->fetchAllPullRequests($repo);

        PullRequest::query()
            ->where('repo_id', $repo->id)
            ->whereNull('first_review_at')
            ->orderBy('number')
            ->each(function (PullRequest $pr) use ($repo): void {
                EnrichPullRequestJob::dispatch($repo->id, $pr->number);
            });
    }
}
