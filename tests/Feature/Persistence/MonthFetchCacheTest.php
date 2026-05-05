<?php

declare(strict_types=1);

namespace Tests\Feature\Persistence;

use App\Models\Repo;
use App\Persistence\Enum\Dataset;
use App\Persistence\MonthFetchCache;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class MonthFetchCacheTest extends TestCase
{
    use RefreshDatabase;

    public function testShouldFetchWhenMonthNeverFetched(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $cache = new MonthFetchCache(CarbonImmutable::create(2026, 5, 15, 12, 0, 0, 'UTC'));

        $this->assertTrue($cache->shouldFetch($repo->id, Dataset::Builds, '2026-04'));
    }

    public function testShouldNotFetchPastMonthMarkedComplete(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $cache = new MonthFetchCache(CarbonImmutable::create(2026, 5, 15, 12, 0, 0, 'UTC'));

        $cache->markComplete($repo->id, Dataset::Builds, '2026-04');

        $this->assertFalse($cache->shouldFetch($repo->id, Dataset::Builds, '2026-04'));
    }

    public function testShouldFetchPartialMonth(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $cache = new MonthFetchCache(CarbonImmutable::create(2026, 5, 15, 12, 0, 0, 'UTC'));

        $cache->markPartial($repo->id, Dataset::Builds, '2026-04');

        $this->assertTrue($cache->shouldFetch($repo->id, Dataset::Builds, '2026-04'));
    }

    public function testShouldAlwaysFetchCurrentMonthEvenIfMarkedComplete(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $cache = new MonthFetchCache(CarbonImmutable::create(2026, 5, 15, 12, 0, 0, 'UTC'));

        $cache->markComplete($repo->id, Dataset::Builds, '2026-05');

        $this->assertTrue($cache->shouldFetch($repo->id, Dataset::Builds, '2026-05'));
    }

    public function testDifferentDatasetsAreIndependent(): void
    {
        $repo = Repo::create(['full_name' => 'your-org/your-repo']);
        $cache = new MonthFetchCache(CarbonImmutable::create(2026, 5, 15, 12, 0, 0, 'UTC'));

        $cache->markComplete($repo->id, Dataset::Builds, '2026-04');

        $this->assertFalse($cache->shouldFetch($repo->id, Dataset::Builds, '2026-04'));
        $this->assertTrue($cache->shouldFetch($repo->id, Dataset::PullRequests, '2026-04'));
    }
}
