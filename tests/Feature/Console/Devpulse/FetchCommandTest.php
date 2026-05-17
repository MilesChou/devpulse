<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use App\Models\Build;
use App\Models\PullRequest;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Saloon\Http\Faking\MockClient;
use Saloon\Http\Faking\MockResponse;
use Tests\Support\CreatesRepoModel;
use Tests\TestCase;

class FetchCommandTest extends TestCase
{
    use CreatesRepoModel;
    use RefreshDatabase;

    protected function setUp(): void
    {
        parent::setUp();
        CarbonImmutable::setTestNow('2026-05-15T00:00:00Z');
    }

    protected function tearDown(): void
    {
        CarbonImmutable::setTestNow(null);
        MockClient::destroyGlobal();
        parent::tearDown();
    }

    public function testFetchesBuildsAndPullRequestsForRepo(): void
    {
        $this->makeRepo('org/repo-a');

        MockClient::global([
            // Travis: list builds
            MockResponse::make([
                'builds' => [$this->travisBuild(id: 1, startedAt: '2026-04-15T10:00:00Z')],
            ]),
            // GitHub commit author bulk（build enrichment）
            MockResponse::make([
                'data' => ['repository' => ['c0' => ['author' => ['user' => ['login' => 'alice']]]]],
            ]),
            // GitHub: list PRs
            MockResponse::make([
                $this->githubPr(number: 42, createdAt: '2026-04-10T10:00:00Z'),
            ]),
            // GitHub PR detail（PR enrichment for #42 - additions / deletions）
            MockResponse::make($this->githubPr(number: 42, createdAt: '2026-04-10T10:00:00Z')),
            // GitHub PR reviews（PR enrichment for #42）
            MockResponse::make([
                'data' => ['repository' => ['pullRequest' => ['reviews' => ['nodes' => []]]]],
            ]),
        ]);

        $this->artisan('devpulse:fetch', [
            'repo' => 'org/repo-a',
            'month' => '2026-04',
        ])
            ->assertSuccessful()
            ->expectsOutputToContain('builds=1、prs=1');

        $this->assertSame(1, Build::query()->count());
        $this->assertSame(1, PullRequest::query()->count());
        $this->assertSame('alice', Build::query()->first()->author_account);
    }

    public function testFailsWhenRepoDoesNotExist(): void
    {
        $this->artisan('devpulse:fetch', [
            'repo' => 'nonexistent/repo',
            'month' => '2026-04',
        ])->assertFailed();
    }

    public function testFailsWhenMonthFormatInvalid(): void
    {
        $this->makeRepo('org/repo-a');
        $this->artisan('devpulse:fetch', [
            'repo' => 'org/repo-a',
            'month' => 'invalid',
        ])->assertFailed();
    }

    /**
     * @return array<string, mixed>
     */
    private function travisBuild(int $id, string $startedAt): array
    {
        return [
            'id' => $id,
            'state' => 'passed',
            'event_type' => 'pull_request',
            'started_at' => $startedAt,
            'duration' => 120,
            'branch' => ['name' => 'feature/foo'],
            'commit' => ['sha' => 'abcdef0123456789'],
            'repository' => ['slug' => 'org/repo-a'],
        ];
    }

    /**
     * @return array<string, mixed>
     */
    private function githubPr(int $number, string $createdAt): array
    {
        return [
            'number' => $number,
            'state' => 'open',
            'draft' => false,
            'user' => ['login' => 'alice'],
            'base' => ['repo' => ['full_name' => 'org/repo-a']],
            'additions' => 30,
            'deletions' => 10,
            'created_at' => $createdAt,
            'merged_at' => null,
            'closed_at' => null,
        ];
    }
}
