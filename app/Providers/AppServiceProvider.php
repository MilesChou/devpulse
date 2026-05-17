<?php

declare(strict_types=1);

namespace App\Providers;

use DevPulse\Ci\CiProvider;
use DevPulse\Vcs\Factory\GitHubPullRequestFactory;
use DevPulse\Vcs\PullRequestFactory;
use App\Infrastructure\Ci\Travis\TravisConnector;
use App\Infrastructure\Ci\Travis\TravisProvider;
use App\Infrastructure\Vcs\GitHub\GitHubConnector;
use App\Infrastructure\Vcs\GitHub\GitHubProvider;
use Illuminate\Cache\RateLimiting\Limit;
use Illuminate\Support\Facades\RateLimiter;
use Illuminate\Support\ServiceProvider;
use RuntimeException;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(TravisConnector::class, function (): TravisConnector {
            $token = config('devpulse.travis_token');
            if (! is_string($token) || $token === '') {
                throw new RuntimeException('TRAVIS_TOKEN is not set, please configure it in .env');
            }

            return new TravisConnector($token);
        });

        $this->app->bind(CiProvider::class, TravisProvider::class);

        $this->app->bind(PullRequestFactory::class, GitHubPullRequestFactory::class);

        $this->app->singleton(GitHubConnector::class, function (): GitHubConnector {
            $token = config('devpulse.github_token');
            if (! is_string($token) || $token === '') {
                throw new RuntimeException('GITHUB_TOKEN is not set, please configure it in .env');
            }

            return new GitHubConnector($token);
        });
    }

    public function boot(): void
    {
        RateLimiter::for(GitHubProvider::RATE_LIMITER, fn () => Limit::perMinute(40));
    }
}
