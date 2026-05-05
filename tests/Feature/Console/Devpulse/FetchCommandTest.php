<?php

declare(strict_types=1);

namespace Tests\Feature\Console\Devpulse;

use App\Models\Build;
use App\Models\Group;
use App\Models\PullRequest;
use App\Models\Repo;
use App\Persistence\Enum\Dataset;
use App\Persistence\MonthFetchCache;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Saloon\Http\Faking\MockClient;
use Saloon\Http\Faking\MockResponse;
use Tests\TestCase;

class FetchCommandTest extends TestCase
{
    use RefreshDatabase;

    protected function setUp(): void
    {
        parent::setUp();
        // 鎖定「現在」為 2026-05，這樣 2026-04 才會被當作「已過月份」走 cache 邏輯
        CarbonImmutable::setTestNow('2026-05-15T00:00:00Z');
    }

    protected function tearDown(): void
    {
        CarbonImmutable::setTestNow(null);
        MockClient::destroyGlobal();
        parent::tearDown();
    }

    public function testFetchesBuildsAndPullRequestsForGroup(): void
    {
        $this->seedGroupWithRepo('team-a', 'org/repo-a');

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
            'group' => 'team-a',
            'month' => '2026-04',
        ])
            ->assertSuccessful()
            ->expectsOutputToContain('builds=1、prs=1');

        $this->assertSame(1, Build::query()->count());
        $this->assertSame(1, PullRequest::query()->count());
        $this->assertSame('alice', Build::query()->first()->author_account);
        // size_bucket 應由 enrichment 寫入（30 + 10 = 40 → XS）
        $this->assertSame('XS', PullRequest::query()->first()->size_bucket);
    }

    public function testSkipsRepoWhenAlreadyComplete(): void
    {
        [, $repo] = $this->seedGroupWithRepo('team-a', 'org/repo-a');
        $cache = new MonthFetchCache();
        $cache->markComplete($repo->id, Dataset::Builds, '2026-04');
        $cache->markComplete($repo->id, Dataset::PullRequests, '2026-04');

        // 不設 mock：如果 fetch 真的打 API，Saloon 會 throw（沒有 mock response）
        MockClient::global([]);

        $this->artisan('devpulse:fetch', [
            'group' => 'team-a',
            'month' => '2026-04',
        ])
            ->assertSuccessful()
            ->expectsOutputToContain('已 complete，跳過');

        $this->assertSame(0, Build::query()->count());
    }

    public function testForceFlagBypassesCache(): void
    {
        [, $repo] = $this->seedGroupWithRepo('team-a', 'org/repo-a');
        $cache = new MonthFetchCache();
        $cache->markComplete($repo->id, Dataset::Builds, '2026-04');
        $cache->markComplete($repo->id, Dataset::PullRequests, '2026-04');

        MockClient::global([
            MockResponse::make(['builds' => [$this->travisBuild(id: 1, startedAt: '2026-04-15T10:00:00Z')]]),
            MockResponse::make([
                'data' => ['repository' => ['c0' => ['author' => ['user' => ['login' => 'alice']]]]],
            ]),
            MockResponse::make([$this->githubPr(number: 99, createdAt: '2026-04-10T10:00:00Z')]),
            MockResponse::make($this->githubPr(number: 99, createdAt: '2026-04-10T10:00:00Z')),
            MockResponse::make([
                'data' => ['repository' => ['pullRequest' => ['reviews' => ['nodes' => []]]]],
            ]),
        ]);

        $this->artisan('devpulse:fetch', [
            'group' => 'team-a',
            'month' => '2026-04',
            '--force' => true,
        ])
            ->assertSuccessful()
            ->expectsOutputToContain('builds=1');

        $this->assertSame(1, Build::query()->count());
    }

    public function testMarksCompleteAfterSuccessfulFetch(): void
    {
        [, $repo] = $this->seedGroupWithRepo('team-a', 'org/repo-a');

        MockClient::global([
            MockResponse::make(['builds' => []]),
            MockResponse::make([]),
        ]);

        $this->artisan('devpulse:fetch', [
            'group' => 'team-a',
            'month' => '2026-04',
        ])->assertSuccessful();

        $cache = new MonthFetchCache();
        $this->assertFalse(
            $cache->shouldFetch($repo->id, Dataset::Builds, '2026-04'),
            'cache 應已標記 builds 為 complete',
        );
        $this->assertFalse(
            $cache->shouldFetch($repo->id, Dataset::PullRequests, '2026-04'),
            'cache 應已標記 PRs 為 complete',
        );
    }

    public function testFailsWhenGroupDoesNotExist(): void
    {
        $this->artisan('devpulse:fetch', [
            'group' => 'nonexistent',
            'month' => '2026-04',
        ])->assertFailed();
    }

    public function testFailsWhenMonthFormatInvalid(): void
    {
        Group::create(['slug' => 'team-a']);
        $this->artisan('devpulse:fetch', [
            'group' => 'team-a',
            'month' => 'invalid',
        ])->assertFailed();
    }

    public function testWarnsWhenGroupHasNoRepos(): void
    {
        Group::create(['slug' => 'empty-team']);

        $this->artisan('devpulse:fetch', [
            'group' => 'empty-team',
            'month' => '2026-04',
        ])
            ->assertSuccessful()
            ->expectsOutputToContain('沒有任何 repo');
    }

    /**
     * @return array{Group, Repo}
     */
    private function seedGroupWithRepo(string $groupSlug, string $repoFullName): array
    {
        $group = Group::create(['slug' => $groupSlug, 'description' => '']);
        $repo = Repo::create(['full_name' => $repoFullName]);
        $group->repos()->attach($repo->id);

        return [$group, $repo];
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
