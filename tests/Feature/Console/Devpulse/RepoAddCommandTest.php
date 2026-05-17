<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use App\Models\Repo;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class RepoAddCommandTest extends TestCase
{
    use RefreshDatabase;

    public function testCreatesRepo(): void
    {
        $this->artisan('devpulse:repo:add', [
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $repo = Repo::query()->where('name', 'your-org/your-repo')->first();
        $this->assertNotNull($repo);
    }

    public function testFailsWhenFullNameMissingSlash(): void
    {
        $this->artisan('devpulse:repo:add', [
            'name' => 'invalid',
        ])->assertFailed();

        $this->assertDatabaseCount('dp_repos', 0);
    }

    public function testIsIdempotentWhenSameRepoAddedAgain(): void
    {
        $this->artisan('devpulse:repo:add', [
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $this->artisan('devpulse:repo:add', [
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $this->assertSame(1, Repo::query()->where('name', 'your-org/your-repo')->count());
    }
}
