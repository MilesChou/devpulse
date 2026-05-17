<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use App\Models\Group;
use App\Models\Repo;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class RepoAddCommandTest extends TestCase
{
    use RefreshDatabase;

    public function testCreatesRepo(): void
    {
        Group::create(['slug' => 'my-team']);

        $this->artisan('devpulse:repo:add', [
            'group' => 'my-team',
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $repo = Repo::query()->where('name', 'your-org/your-repo')->first();
        $this->assertNotNull($repo);
    }

    public function testFailsWhenGroupDoesNotExist(): void
    {
        $this->artisan('devpulse:repo:add', [
            'group' => 'nonexistent',
            'name' => 'your-org/your-repo',
        ])->assertFailed();

        $this->assertDatabaseCount('dp_repos', 0);
    }

    public function testFailsWhenFullNameMissingSlash(): void
    {
        Group::create(['slug' => 'my-team']);

        $this->artisan('devpulse:repo:add', [
            'group' => 'my-team',
            'name' => 'invalid',
        ])->assertFailed();

        $this->assertDatabaseCount('dp_repos', 0);
    }

    public function testIsIdempotentWhenSameRepoAddedAgain(): void
    {
        Group::create(['slug' => 'my-team']);

        $this->artisan('devpulse:repo:add', [
            'group' => 'my-team',
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $this->artisan('devpulse:repo:add', [
            'group' => 'my-team',
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $this->assertSame(1, Group::query()->where('slug', 'my-team')->first()->repos()->count());
    }

    public function testReusesExistingRepoAcrossGroups(): void
    {
        $teamA = Group::create(['slug' => 'team-a']);
        $teamB = Group::create(['slug' => 'team-b']);

        $this->artisan('devpulse:repo:add', [
            'group' => 'team-a',
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $this->artisan('devpulse:repo:add', [
            'group' => 'team-b',
            'name' => 'your-org/your-repo',
        ])->assertSuccessful();

        $this->assertSame(1, Repo::query()->where('name', 'your-org/your-repo')->count());
        $this->assertCount(1, $teamA->fresh()->repos);
        $this->assertCount(1, $teamB->fresh()->repos);
    }
}
