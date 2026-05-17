<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Fetching\FetchOrchestrator;
use App\Models\Repo;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\Middleware\RateLimited;
use Illuminate\Queue\SerializesModels;
use RuntimeException;

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
        return [new RateLimited('github')];
    }

    public function handle(FetchOrchestrator $orchestrator): void
    {
        $repo = Repo::query()->find($this->repoId);
        if ($repo === null) {
            throw new RuntimeException("repo `{$this->repoId}` 不存在");
        }

        $found = $orchestrator->enrichOnePullRequestByNumber($repo, $this->prNumber);
        if (!$found) {
            throw new RuntimeException("PR #{$this->prNumber} 在 repo `{$repo->name}` 中不存在");
        }
    }
}
