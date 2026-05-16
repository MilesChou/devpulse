<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use App\Models\Group;
use App\Models\Member;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class MemberAddCommandTest extends TestCase
{
    use RefreshDatabase;

    public function testCreatesMemberAndAttachesToGroup(): void
    {
        Group::create(['slug' => 'my-team']);

        $this->artisan('devpulse:member:add', [
            'group' => 'my-team',
            'github_account' => 'alice',
            'display_name' => 'Alice',
        ])->assertSuccessful();

        $member = Member::query()->where('github_account', 'alice')->first();
        $this->assertNotNull($member);
        $this->assertSame('Alice', $member->display_name);
        $this->assertTrue(Group::query()->where('slug', 'my-team')->first()->members->contains($member));
    }

    public function testReusesExistingMemberWhenAddingToAnotherGroup(): void
    {
        $teamA = Group::create(['slug' => 'team-a']);
        $teamB = Group::create(['slug' => 'team-b']);

        $this->artisan('devpulse:member:add', [
            'group' => 'team-a',
            'github_account' => 'alice',
            'display_name' => 'Alice',
        ])->assertSuccessful();

        $this->artisan('devpulse:member:add', [
            'group' => 'team-b',
            'github_account' => 'alice',
            'display_name' => 'Alice (other label)',
        ])->assertSuccessful();

        $this->assertSame(1, Member::query()->where('github_account', 'alice')->count());
        $this->assertCount(1, $teamA->fresh()->members);
        $this->assertCount(1, $teamB->fresh()->members);
    }

    public function testFailsWhenGroupDoesNotExist(): void
    {
        $this->artisan('devpulse:member:add', [
            'group' => 'nonexistent',
            'github_account' => 'alice',
            'display_name' => 'Alice',
        ])->assertFailed();

        $this->assertDatabaseCount('dp_members', 0);
    }

    public function testIsIdempotentWhenSameMemberAddedAgain(): void
    {
        Group::create(['slug' => 'my-team']);

        $this->artisan('devpulse:member:add', [
            'group' => 'my-team',
            'github_account' => 'alice',
            'display_name' => 'Alice',
        ])->assertSuccessful();

        $this->artisan('devpulse:member:add', [
            'group' => 'my-team',
            'github_account' => 'alice',
            'display_name' => 'Alice',
        ])->assertSuccessful();

        $this->assertSame(1, Group::query()->where('slug', 'my-team')->first()->members()->count());
    }
}
