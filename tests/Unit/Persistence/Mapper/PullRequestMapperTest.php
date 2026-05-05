<?php

declare(strict_types=1);

namespace Tests\Unit\Persistence\Mapper;

use App\Domain\Vcs\PullRequestStatus;
use App\Domain\Vcs\PullRequestSummary;
use App\Persistence\Mapper\PullRequestMapper;
use Carbon\CarbonImmutable;
use PHPUnit\Framework\TestCase;

class PullRequestMapperTest extends TestCase
{
    public function testToAttributesIncludesDerivedDimensions(): void
    {
        $createdAt = CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');
        $vo = new PullRequestSummary(
            repoFullName: 'your-org/your-repo',
            number: 42,
            authorAccount: 'alice',
            status: PullRequestStatus::Open,
            additions: 100,
            deletions: 50,
            createdAt: $createdAt,
            readyAt: $createdAt,
            mergedAt: null,
            closedAt: null,
        );

        $attributes = (new PullRequestMapper())->toAttributes($vo, repoId: 7, rawPayload: []);

        $this->assertSame(7, $attributes['repo_id']);
        $this->assertSame(42, $attributes['number']);
        $this->assertSame('open', $attributes['status']);
        $this->assertSame(150, $attributes['total_changed_lines']);
        $this->assertFalse($attributes['is_draft']);
    }

    public function testToAttributesMarksDraftWhenReadyAtNull(): void
    {
        $vo = new PullRequestSummary(
            repoFullName: 'your-org/your-repo',
            number: 42,
            authorAccount: 'alice',
            status: PullRequestStatus::Open,
            additions: 1,
            deletions: 0,
            createdAt: CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC'),
            readyAt: null,
            mergedAt: null,
            closedAt: null,
        );

        $attributes = (new PullRequestMapper())->toAttributes($vo, repoId: 1, rawPayload: []);
        $this->assertTrue($attributes['is_draft']);
    }
}
