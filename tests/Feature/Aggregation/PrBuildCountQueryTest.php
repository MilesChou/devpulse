<?php

declare(strict_types=1);

namespace Tests\Feature\Aggregation;

use App\Aggregation\PrBuildCountQuery;
use DevPulse\Shared\MonthRange;
use App\Models\Build;
use App\Models\Group;
use App\Models\Repo;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class PrBuildCountQueryTest extends TestCase
{
    use RefreshDatabase;

    private PrBuildCountQuery $query;

    protected function setUp(): void
    {
        parent::setUp();
        $this->query = new PrBuildCountQuery();
    }

    public function testCountsBuildsPerPr(): void
    {
        [$group, $repo] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        $this->insertBuild($repo->id, prNumber: 1, date: '2026-04-10');
        $this->insertBuild($repo->id, prNumber: 1, date: '2026-04-11');
        $this->insertBuild($repo->id, prNumber: 2, date: '2026-04-12');

        $results = $this->query->run($group, MonthRange::fromString('2026-04'));

        $this->assertSame(2, $results->count());

        $pr1 = $results->first(fn ($r) => $r->prNumber->value === 1);
        $this->assertNotNull($pr1);
        $this->assertSame(2, $pr1->buildCount);
        $this->assertSame('org/repo-a', (string)$pr1->repoFullName);

        $pr2 = $results->first(fn ($r) => $r->prNumber->value === 2);
        $this->assertNotNull($pr2);
        $this->assertSame(1, $pr2->buildCount);
    }

    public function testExcludesBuildsFromOtherGroups(): void
    {
        [$groupA, $repoA] = $this->setupGroupWithRepo('team-a', 'org/repo-a');
        [, $repoB] = $this->setupGroupWithRepo('team-b', 'org/repo-b');

        $this->insertBuild($repoA->id, prNumber: 1, date: '2026-04-10');
        $this->insertBuild($repoB->id, prNumber: 1, date: '2026-04-10');

        $results = $this->query->run($groupA, MonthRange::fromString('2026-04'));

        $this->assertSame(1, $results->count());
        $this->assertSame('org/repo-a', (string)$results->first()->repoFullName);
    }

    public function testExcludesBuildsOutsideMonth(): void
    {
        [$group, $repo] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        $this->insertBuild($repo->id, prNumber: 1, date: '2026-03-31');
        $this->insertBuild($repo->id, prNumber: 1, date: '2026-04-01');
        $this->insertBuild($repo->id, prNumber: 1, date: '2026-04-30');
        $this->insertBuild($repo->id, prNumber: 1, date: '2026-05-01');

        $results = $this->query->run($group, MonthRange::fromString('2026-04'));

        $this->assertSame(1, $results->count());
        $this->assertSame(2, $results->first()->buildCount);
    }

    public function testExcludesNonPrBuilds(): void
    {
        [$group, $repo] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        $this->insertBuild($repo->id, prNumber: 1, date: '2026-04-10');
        $this->insertBuild($repo->id, prNumber: null, date: '2026-04-10');

        $results = $this->query->run($group, MonthRange::fromString('2026-04'));

        $this->assertSame(1, $results->count());
        $this->assertSame(1, $results->first()->prNumber->value);
    }

    public function testReturnsEmptyWhenNoBuilds(): void
    {
        [$group] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        $results = $this->query->run($group, MonthRange::fromString('2026-04'));

        $this->assertTrue($results->isEmpty());
    }

    /**
     * @return array{Group, Repo}
     */
    private function setupGroupWithRepo(string $groupSlug, string $repoFullName): array
    {
        $group = Group::create(['slug' => $groupSlug, 'description' => '']);
        $repo = Repo::create(['full_name' => $repoFullName]);
        $group->repos()->attach($repo->id);

        return [$group, $repo];
    }

    private function insertBuild(int $repoId, ?int $prNumber, string $date): void
    {
        Build::create([
            'repo_id' => $repoId,
            'external_id' => uniqid(),
            'commit_sha' => str_repeat('a', 40),
            'author_account' => 'alice',
            'pr_number' => $prNumber,
            'status' => 'PASSED',
            'trigger' => 'PULL_REQUEST',
            'branch' => 'feature/test',
            'is_post_merge' => false,
            'is_pull_request' => $prNumber !== null,
            'is_deploy_event' => false,
            'is_failure' => false,
            'started_at' => CarbonImmutable::parse($date . 'T10:00:00Z'),
            'duration_seconds' => 120,
            'raw_payload' => [],
        ]);
    }
}
