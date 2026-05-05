<?php

declare(strict_types=1);

namespace Tests\Feature\Aggregation;

use App\Aggregation\ReviewLatencyQuery;
use App\Domain\Shared\MonthRange;
use App\Domain\Vcs\PullRequestStatus;
use App\Models\Group;
use App\Models\PullRequest;
use App\Models\Repo;
use Carbon\CarbonImmutable;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class ReviewLatencyQueryTest extends TestCase
{
    use RefreshDatabase;

    private Group $group;
    private Repo $repo;

    protected function setUp(): void
    {
        parent::setUp();
        $this->group = Group::create(['slug' => 'team-a', 'description' => '']);
        $this->repo = Repo::create(['full_name' => 'org/repo']);
        $this->group->repos()->attach($this->repo->id);
    }

    public function testCalculatesLatencyForReviewedPr(): void
    {
        $readyAt = CarbonImmutable::parse('2026-04-10T10:00:00Z');
        $firstReviewAt = CarbonImmutable::parse('2026-04-10T12:00:00Z');

        $this->insertPr(number: 1, readyAt: $readyAt, firstReviewAt: $firstReviewAt, sizeBucket: 'S');

        $results = (new ReviewLatencyQuery())->run($this->group, MonthRange::fromString('2026-04'));

        $this->assertCount(1, $results);
        $pr = $results->first();
        $this->assertEqualsWithDelta(2.0, $pr->latencyHours, 0.01);
        $this->assertFalse($pr->isLowerBound);
        $this->assertSame('S', $pr->sizeBucket);
    }

    public function testReturnsLowerBoundForUnreviewedPr(): void
    {
        $readyAt = CarbonImmutable::parse('2026-04-10T10:00:00Z');
        $this->insertPr(number: 2, readyAt: $readyAt, firstReviewAt: null, sizeBucket: 'XS');

        $clock = CarbonImmutable::parse('2026-05-05T00:00:00Z');
        $results = (new ReviewLatencyQuery($clock))->run($this->group, MonthRange::fromString('2026-04'));

        $this->assertCount(1, $results);
        $pr = $results->first();
        $this->assertTrue($pr->isLowerBound);
        $this->assertGreaterThan(0, $pr->latencyHours);
    }

    public function testExcludesDraftPrs(): void
    {
        $this->insertPr(number: 3, readyAt: null, firstReviewAt: null, sizeBucket: null);

        $results = (new ReviewLatencyQuery())->run($this->group, MonthRange::fromString('2026-04'));

        $this->assertCount(0, $results);
    }

    public function testExcludesPrsFromOtherGroups(): void
    {
        $otherGroup = Group::create(['slug' => 'team-b', 'description' => '']);
        $otherRepo = Repo::create(['full_name' => 'org/other']);
        $otherGroup->repos()->attach($otherRepo->id);

        $this->insertPrForRepo(
            $otherRepo->id,
            number: 4,
            readyAt: CarbonImmutable::parse('2026-04-10T10:00:00Z'),
            firstReviewAt: CarbonImmutable::parse('2026-04-10T12:00:00Z'),
            sizeBucket: 'S',
        );

        $results = (new ReviewLatencyQuery())->run($this->group, MonthRange::fromString('2026-04'));
        $this->assertCount(0, $results);
    }

    private function insertPr(
        int $number,
        ?CarbonImmutable $readyAt,
        ?CarbonImmutable $firstReviewAt,
        ?string $sizeBucket,
    ): void {
        $this->insertPrForRepo($this->repo->id, $number, $readyAt, $firstReviewAt, $sizeBucket);
    }

    private function insertPrForRepo(
        int $repoId,
        int $number,
        ?CarbonImmutable $readyAt,
        ?CarbonImmutable $firstReviewAt,
        ?string $sizeBucket,
    ): void {
        PullRequest::create([
            'repo_id' => $repoId,
            'number' => $number,
            'author_account' => 'alice',
            'status' => PullRequestStatus::Open->value,
            'additions' => 50,
            'deletions' => 10,
            'total_changed_lines' => 60,
            'size_bucket' => $sizeBucket,
            'is_draft' => $readyAt === null,
            'pr_created_at' => CarbonImmutable::parse('2026-04-01T09:00:00Z'),
            'ready_at' => $readyAt,
            'first_review_at' => $firstReviewAt,
            'merged_at' => null,
            'closed_at' => null,
            'raw_payload' => [],
        ]);
    }
}
