<?php

declare(strict_types=1);

namespace App\Providers;

use App\Domain\Ci\CiProvider;
use App\Domain\Ci\Travis\TravisConnector;
use App\Domain\Ci\Travis\TravisProvider;
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
    }

    public function boot(): void
    {
        //
    }
}
