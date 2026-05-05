<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Vcs;

use App\Domain\Shared\RepoFullName;
use App\Domain\Vcs\PullRequestStatus;
use App\Domain\Vcs\PullRequestSummary;
use Carbon\CarbonImmutable;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class PullRequestSummaryTest extends TestCase
{
    public function testBuildsValidInstance(): void
    {
        $pr = $this->build();
        $this->assertSame('your-org/your-repo', (string)$pr->repoFullName);
        $this->assertSame(42, $pr->number);
        $this->assertSame(PullRequestStatus::Open, $pr->status);
    }

    public function testThrowsWhenNumberZero(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('number');
        $this->build(number: 0);
    }

    public function testThrowsWhenAuthorAccountIsEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('authorAccount');
        $this->build(authorAccount: '');
    }

    public function testThrowsWhenAdditionsNegative(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('additions');
        $this->build(additions: -1);
    }

    public function testThrowsWhenMergedWithoutMergedAt(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('mergedAt');
        $this->build(status: PullRequestStatus::Merged, mergedAt: null);
    }

    public function testThrowsWhenClosedWithoutClosedAt(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('closedAt');
        $this->build(status: PullRequestStatus::Closed, closedAt: null);
    }

    public function testIsDraftWhenReadyAtIsNull(): void
    {
        $this->assertTrue($this->buildWithReadyAtOverride(null)->isDraft());
        $this->assertFalse($this->build()->isDraft());
    }

    private function buildWithReadyAtOverride(?CarbonImmutable $readyAt): PullRequestSummary
    {
        $createdAt = CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');

        return new PullRequestSummary(
            repoFullName: new RepoFullName('your-org/your-repo'),
            number: 42,
            authorAccount: 'alice',
            status: PullRequestStatus::Open,
            additions: 10,
            deletions: 5,
            createdAt: $createdAt,
            readyAt: $readyAt,
            mergedAt: null,
            closedAt: null,
        );
    }

    public function testTotalChangedLines(): void
    {
        $this->assertSame(150, $this->build(additions: 100, deletions: 50)->totalChangedLines());
    }

    public function testFromGitHubRawTranslatesOpenPr(): void
    {
        $pr = PullRequestSummary::fromGitHubRaw($this->payload());
        $this->assertSame(PullRequestStatus::Open, $pr->status);
        $this->assertFalse($pr->isDraft());
        $this->assertNull($pr->mergedAt);
    }

    public function testFromGitHubRawTranslatesMergedPr(): void
    {
        $pr = PullRequestSummary::fromGitHubRaw($this->payload([
            'state' => 'closed',
            'merged_at' => '2026-04-15T11:00:00Z',
            'closed_at' => '2026-04-15T11:00:00Z',
        ]));
        $this->assertSame(PullRequestStatus::Merged, $pr->status);
        $this->assertNotNull($pr->mergedAt);
    }

    public function testFromGitHubRawTranslatesDraftPr(): void
    {
        $pr = PullRequestSummary::fromGitHubRaw($this->payload(['draft' => true]));
        $this->assertTrue($pr->isDraft());
        $this->assertNull($pr->readyAt);
    }

    public function testFromGitHubRawThrowsWhenRepoMissing(): void
    {
        $payload = $this->payload();
        unset($payload['base']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('base.repo.full_name');
        PullRequestSummary::fromGitHubRaw($payload);
    }

    public function testFromGitHubRawThrowsWhenNumberMissing(): void
    {
        $payload = $this->payload();
        unset($payload['number']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('number');
        PullRequestSummary::fromGitHubRaw($payload);
    }

    public function testFromGitHubRawParsesTimesAsUtc(): void
    {
        $pr = PullRequestSummary::fromGitHubRaw($this->payload([
            'created_at' => '2026-04-15T10:00:00Z',
        ]));

        $this->assertSame('UTC', $pr->createdAt->getTimezone()->getName());
        $this->assertSame('2026-04-15 10:00:00', $pr->createdAt->format('Y-m-d H:i:s'));
    }

    private function build(
        int $number = 42,
        string $authorAccount = 'alice',
        PullRequestStatus $status = PullRequestStatus::Open,
        int $additions = 10,
        int $deletions = 5,
        ?CarbonImmutable $createdAt = null,
        ?CarbonImmutable $readyAt = null,
        ?CarbonImmutable $mergedAt = null,
        ?CarbonImmutable $closedAt = null,
    ): PullRequestSummary {
        $createdAt ??= CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');

        return new PullRequestSummary(
            repoFullName: new RepoFullName('your-org/your-repo'),
            number: $number,
            authorAccount: $authorAccount,
            status: $status,
            additions: $additions,
            deletions: $deletions,
            createdAt: $createdAt,
            readyAt: $readyAt ?? $createdAt,
            mergedAt: $mergedAt,
            closedAt: $closedAt,
        );
    }

    /**
     * @param array<string, mixed> $overrides
     * @return array<string, mixed>
     */
    private function payload(array $overrides = []): array
    {
        return array_replace([
            'number' => 42,
            'state' => 'open',
            'draft' => false,
            'additions' => 10,
            'deletions' => 5,
            'created_at' => '2026-04-15T10:00:00Z',
            'merged_at' => null,
            'closed_at' => null,
            'user' => ['login' => 'alice'],
            'base' => ['repo' => ['full_name' => 'your-org/your-repo']],
        ], $overrides);
    }
}
