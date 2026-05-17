<?php

declare(strict_types=1);

namespace App\Jobs;

use App\Infrastructure\Vcs\GitHub\GitHubProvider;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\Middleware\RateLimited;
use Illuminate\Queue\SerializesModels;

abstract class GitHubJob implements ShouldQueue
{
    use Dispatchable;
    use InteractsWithQueue;
    use Queueable;
    use SerializesModels;

    public int $tries = 3;

    public int $timeout = 600;

    /**
     * @return list<object>
     */
    public function middleware(): array
    {
        return [new RateLimited(GitHubProvider::RATE_LIMITER)];
    }
}
