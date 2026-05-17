<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Fetching\FetchOrchestrator;
use App\Models\PullRequest;
use App\Models\Repo;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\Middleware\RateLimited;
use Illuminate\Queue\SerializesModels;
use RuntimeException;

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
        return [new RateLimited('github')];
    }

    public function handle(FetchOrchestrator $orchestrator): void
    {
        $repo = Repo::query()->find($this->repoId);
        if ($repo === null) {
            throw new RuntimeException("repo `{$this->repoId}` 不存在");
        }

        $outcome = $orchestrator->fetchAllPullRequests($repo);

        if ($outcome->error !== null) {
            throw new RuntimeException("fetch all PRs failed for {$outcome->repoFullName}: {$outcome->error}");
        }

        PullRequest::query()
            ->where('repo_id', $repo->id)
            ->orderBy('number')
            ->each(function (PullRequest $pr): void {
                EnrichPullRequestJob::dispatch($pr->repo_id, $pr->number);
            });
    }
}
