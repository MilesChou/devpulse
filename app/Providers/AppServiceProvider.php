<?php

declare(strict_types=1);

namespace App\Providers;

use App\Domain\Ci\CiProvider;
use App\Domain\Ci\Travis\TravisConnector;
use App\Domain\Ci\Travis\TravisProvider;
use App\Domain\Vcs\Filter\BotFilter;
use App\Domain\Vcs\GitHub\GitHubConnector;
use Illuminate\Support\ServiceProvider;
use RuntimeException;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(TravisConnector::class, function (): TravisConnector {
            $token = config('devpulse.travis_token');
            if (! is_string($token) || $token === '') {
                throw new RuntimeException('TRAVIS_TOKEN 未設定，請在 .env 中設定');
            }

            return new TravisConnector($token);
        });

        $this->app->bind(CiProvider::class, TravisProvider::class);

        $this->app->singleton(GitHubConnector::class, function (): GitHubConnector {
            $token = config('devpulse.github_token');
            if (! is_string($token) || $token === '') {
                throw new RuntimeException('GITHUB_TOKEN 未設定，請在 .env 中設定');
            }

            return new GitHubConnector($token);
        });

        $this->app->singleton(BotFilter::class, function (): BotFilter {
            $excluded = config('devpulse.excluded_bots');

            return new BotFilter(is_array($excluded) ? array_values(array_filter($excluded, 'is_string')) : []);
        });
    }

    public function boot(): void
    {
        //
    }
}
