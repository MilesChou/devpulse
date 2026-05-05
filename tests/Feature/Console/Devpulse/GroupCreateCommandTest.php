<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use App\Models\Group;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class GroupCreateCommandTest extends TestCase
{
    use RefreshDatabase;

    public function testCreatesGroupWithSlugOnly(): void
    {
        $this->artisan('devpulse:group:create', ['slug' => 'my-team'])
            ->assertSuccessful();

        $group = Group::query()->where('slug', 'my-team')->first();
        $this->assertNotNull($group);
        $this->assertNull($group->description);
    }

    public function testCreatesGroupWithDescription(): void
    {
        $this->artisan('devpulse:group:create', [
            'slug' => 'my-team',
            '--description' => 'demo group',
        ])->assertSuccessful();

        $this->assertSame('demo group', Group::query()->where('slug', 'my-team')->value('description'));
    }

    public function testRejectsInvalidSlugCharacters(): void
    {
        $this->artisan('devpulse:group:create', ['slug' => 'Bad Slug!'])
            ->assertFailed();

        $this->assertDatabaseCount('groups', 0);
    }

    public function testRejectsUppercaseSlug(): void
    {
        $this->artisan('devpulse:group:create', ['slug' => 'MyTeam'])
            ->assertFailed();

        $this->assertDatabaseCount('groups', 0);
    }

    public function testRejectsDuplicateSlug(): void
    {
        Group::create(['slug' => 'my-team']);

        $this->artisan('devpulse:group:create', ['slug' => 'my-team'])
            ->assertFailed();

        $this->assertSame(1, Group::query()->where('slug', 'my-team')->count());
    }
}
