<?php

declare(strict_types=1);

namespace Tests\Feature\Aggregation;

use App\Aggregation\BuildFailureRateQuery;
use App\Aggregation\Filter\BuildEventFilter;
use DevPulse\Ci\BuildStatus;
use DevPulse\Ci\BuildTrigger;
use DevPulse\Shared\MonthRange;
use App\Models\Build;
use App\Models\Group;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\Support\CreatesRepoModel;
use Tests\TestCase;

class BuildFailureRateQueryTest extends TestCase
{
    use RefreshDatabase;
    use CreatesRepoModel;

    private BuildFailureRateQuery $query;

    protected function setUp(): void
    {
        parent::setUp();
        $this->query = new BuildFailureRateQuery(new BuildEventFilter());
    }

    public function testAggregatesFailureRateByAuthorAndRepo(): void
    {
        [$groupA, $repoA] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        $this->insertBuild($repoA->id, 'alice', '2026-04-10', BuildStatus::PASSED);
        $this->insertBuild($repoA->id, 'alice', '2026-04-11', BuildStatus::FAILED);
        $this->insertBuild($repoA->id, 'alice', '2026-04-12', BuildStatus::PASSED);
        $this->insertBuild($repoA->id, 'bob', '2026-04-10', BuildStatus::FAILED);
        $this->insertBuild($repoA->id, 'bob', '2026-04-11', BuildStatus::FAILED);

        $results = $this->query->run($groupA, MonthRange::fromString('2026-04'));

        $alice = $results->firstWhere('authorAccount', 'alice');
        $bob = $results->firstWhere('authorAccount', 'bob');

        $this->assertNotNull($alice);
        $this->assertSame(3, $alice->total);
        $this->assertSame(1, $alice->failures);
        $this->assertEqualsWithDelta(1 / 3, $alice->rate, 1e-9);

        $this->assertNotNull($bob);
        $this->assertSame(2, $bob->total);
        $this->assertSame(2, $bob->failures);
        $this->assertEqualsWithDelta(1.0, $bob->rate, 1e-9);
    }

    public function testExcludesBuildsFromOtherGroups(): void
    {
        [$groupA, $repoA] = $this->setupGroupWithRepo('team-a', 'org/repo-a');
        [, $repoB] = $this->setupGroupWithRepo('team-b', 'org/repo-b');

        $this->insertBuild($repoA->id, 'alice', '2026-04-10', BuildStatus::FAILED);
        $this->insertBuild($repoB->id, 'alice', '2026-04-10', BuildStatus::FAILED);

        $results = $this->query->run($groupA, MonthRange::fromString('2026-04'));

        // group-a 只能看到 repo-a 的資料
        $this->assertSame(1, $results->count());
        $this->assertSame('org/repo-a', (string)$results->first()->repoFullName);
    }

    public function testExcludesBuildsOutsideMonth(): void
    {
        [$groupA, $repoA] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        $this->insertBuild($repoA->id, 'alice', '2026-03-31', BuildStatus::FAILED);
        $this->insertBuild($repoA->id, 'alice', '2026-04-01', BuildStatus::PASSED);
        $this->insertBuild($repoA->id, 'alice', '2026-04-30', BuildStatus::PASSED);
        $this->insertBuild($repoA->id, 'alice', '2026-05-01', BuildStatus::FAILED);

        $results = $this->query->run($groupA, MonthRange::fromString('2026-04'));

        $alice = $results->firstWhere('authorAccount', 'alice');
        $this->assertNotNull($alice);
        $this->assertSame(2, $alice->total);
        $this->assertSame(0, $alice->failures);
    }

    public function testDefaultExcludesPostMergeBuilds(): void
    {
        [$groupA, $repoA] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        // is_post_merge = true 的 build 應被排除
        $this->insertBuild($repoA->id, 'alice', '2026-04-10', BuildStatus::FAILED, isPostMerge: true);
        $this->insertBuild($repoA->id, 'alice', '2026-04-11', BuildStatus::PASSED, isPostMerge: false);

        $results = $this->query->run($groupA, MonthRange::fromString('2026-04'));

        $alice = $results->firstWhere('authorAccount', 'alice');
        $this->assertNotNull($alice);
        $this->assertSame(1, $alice->total);
        $this->assertSame(0, $alice->failures);
    }

    public function testIncludesPostMergeWhenFilterAllows(): void
    {
        [$groupA, $repoA] = $this->setupGroupWithRepo('team-a', 'org/repo-a');

        $this->insertBuild($repoA->id, 'alice', '2026-04-10', BuildStatus::FAILED, isPostMerge: true);
        $this->insertBuild($repoA->id, 'alice', '2026-04-11', BuildStatus::PASSED, isPostMerge: false);

        $query = new BuildFailureRateQuery(new BuildEventFilter(includePostMerge: true));
        $results = $query->run($groupA, MonthRange::fromString('2026-04'));

        $alice = $results->firstWhere('authorAccount', 'alice');
        $this->assertNotNull($alice);
        $this->assertSame(2, $alice->total);
        $this->assertSame(1, $alice->failures);
    }

    /**
     * @return array{Group, Repo}
     */
    private function setupGroupWithRepo(string $groupSlug, string $repoFullName): array
    {
        $group = Group::create(['slug' => $groupSlug, 'description' => '']);
        $repo = $this->makeRepo($repoFullName);
        $group->repos()->attach($repo->id);

        return [$group, $repo];
    }

    private function insertBuild(
        string $repoId,
        string $authorAccount,
        string $date,
        BuildStatus $status,
        bool $isPostMerge = false,
        bool $isDeployEvent = false,
    ): void {
        Build::create([
            'repo_id' => $repoId,
            'external_id' => uniqid(),
            'commit_sha' => str_repeat('a', 40),
            'author_account' => $authorAccount,
            'pr_number' => null,
            'status' => $status->name,
            'trigger' => $isPostMerge ? BuildTrigger::POST_MERGE->name : BuildTrigger::PULL_REQUEST->name,
            'branch' => $isPostMerge ? 'master' : 'feature/test',
            'is_post_merge' => $isPostMerge,
            'is_pull_request' => ! $isPostMerge && ! $isDeployEvent,
            'is_deploy_event' => $isDeployEvent,
            'is_failure' => $status->isFailure(),
            'started_at' => CarbonImmutable::parse($date . 'T10:00:00Z'),
            'duration_seconds' => 120,
            'raw_payload' => [],
        ]);
    }
}
