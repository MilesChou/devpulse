<?php

declare(strict_types=1);

namespace Tests\Unit\Persistence\Mapper;

use App\Domain\Shared\RepoId;
use App\Domain\Vcs\Author;
use App\Domain\Vcs\PullRequestNumber;
use App\Domain\Vcs\PullRequestStatus;
use App\Domain\Vcs\PullRequest;
use App\Persistence\Mapper\PullRequestMapper;
use Carbon\CarbonImmutable;
use PHPUnit\Framework\TestCase;

class PullRequestMapperTest extends TestCase
{
    public function testMapperIncludesDerivedDimensions(): void
    {
        $createdAt = CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');
        $vo = new PullRequest(
            repoId: new RepoId(7),
            number: new PullRequestNumber(42),
            author: new Author('alice'),
            status: PullRequestStatus::Open,
            additions: 100,
            deletions: 50,
            createdAt: $createdAt,
            readyAt: $createdAt,
            mergedAt: null,
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
            repoId: new RepoId(1),
            number: new PullRequestNumber(42),
            author: new Author('alice'),
            status: PullRequestStatus::Open,
            additions: 1,
            deletions: 0,
            createdAt: CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC'),
            readyAt: null,
            mergedAt: null,
            closedAt: null,
        );

        $attributes = (new PullRequestMapper())->toAttributes($vo);
        $this->assertTrue($attributes['is_draft']);
    }
}
