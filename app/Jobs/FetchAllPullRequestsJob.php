<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Fetching\FetchOrchestrator;
use App\Models\PullRequest;
use App\Models\Repo;

/**
 * Fetches the full PR history for a repo (state=all) and upserts the list,
 * then dispatches one EnrichPullRequestJob per unenriched PR.
 */
class FetchAllPullRequestsJob extends GitHubJob
{
    public function __construct(public readonly string $repoId)
    {
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
