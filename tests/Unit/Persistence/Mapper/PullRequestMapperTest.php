<?php

declare(strict_types=1);

namespace Tests\Unit\Persistence\Mapper;

use DevPulse\Shared\RepoId;
use DevPulse\Vcs\Author;
use DevPulse\Vcs\ChangeStats;
use DevPulse\Vcs\Platform;
use DevPulse\Vcs\PullRequest;
use DevPulse\Vcs\PullRequestId;
use DevPulse\Vcs\PullRequestNumber;
use DevPulse\Vcs\PullRequestStatus;
use App\Persistence\Mapper\PullRequestMapper;
use Carbon\CarbonImmutable;
use PHPUnit\Framework\TestCase;

class PullRequestMapperTest extends TestCase
{
    public function testMapperIncludesDerivedDimensions(): void
    {
        $createdAt = CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');
        $vo = new PullRequest(
            id: new PullRequestId('01JTEST000000000000000000D'),
            platform: Platform::GitHub,
            repoId: new RepoId(7),
            number: new PullRequestNumber(42),
            author: new Author('alice'),
            status: PullRequestStatus::Open,
            changes: new ChangeStats(100, 50),
            createdAt: $createdAt,
            readyAt: $createdAt,
            closedAt: null,
        );

        $attributes = new PullRequestMapper()->toAttributes($vo);

        $this->assertSame(7, $attributes['repo_id']);
        $this->assertSame(42, $attributes['number']);
        $this->assertSame('open', $attributes['status']);
        $this->assertSame(150, $attributes['total_changed_lines']);
        $this->assertFalse($attributes['is_draft']);
    }

    public function testMapperMarksDraftWhenReadyAtNull(): void
    {
        $vo = new PullRequest(
            id: new PullRequestId('01JTEST000000000000000000E'),
            platform: Platform::GitHub,
            repoId: new RepoId(1),
            number: new PullRequestNumber(42),
            author: new Author('alice'),
            status: PullRequestStatus::Open,
            changes: new ChangeStats(1, 0),
            createdAt: CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC'),
            readyAt: null,
            closedAt: null,
        );

        $attributes = new PullRequestMapper()->toAttributes($vo);
        $this->assertTrue($attributes['is_draft']);
    }
}
