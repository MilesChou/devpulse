<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use App\Jobs\FetchAllPullRequestsJob;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Illuminate\Support\Facades\Bus;
use Tests\Support\CreatesRepoModel;
use Tests\TestCase;

class FetchCommandTest extends TestCase
{
    use CreatesRepoModel;
    use RefreshDatabase;

    public function testDispatchesFetchAllPullRequestsJobForRepo(): void
    {
        Bus::fake();
        $repo = $this->makeRepo('org/repo-a');

        $this->artisan('devpulse:fetch', ['repo' => 'org/repo-a'])
            ->assertSuccessful();

        Bus::assertDispatched(
            FetchAllPullRequestsJob::class,
            fn (FetchAllPullRequestsJob $job): bool => $job->repoId === $repo->id,
        );
    }

    public function testFailsWhenRepoDoesNotExist(): void
    {
        Bus::fake();

        $this->artisan('devpulse:fetch', ['repo' => 'nonexistent/repo'])
            ->assertFailed();

        Bus::assertNothingDispatched();
    }
}
