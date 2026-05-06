<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Vcs;

use App\Domain\Shared\RepoId;
use App\Domain\Vcs\Author;
use App\Domain\Vcs\Factory\GitHubPullRequestFactory;
use App\Domain\Vcs\PullRequestNumber;
use App\Domain\Vcs\PullRequestStatus;
use App\Domain\Vcs\PullRequest;
use Carbon\CarbonImmutable;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class PullRequestSummaryTest extends TestCase
{
    public function testBuildsValidInstance(): void
    {
        $pr = $this->build();
        $this->assertSame(7, $pr->repoId->toInt());
        $this->assertSame(42, $pr->number->toInt());
        $this->assertSame(PullRequestStatus::Open, $pr->status);
    }

    public function testThrowsWhenNumberZero(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('PullRequestNumber');
        $this->build(number: 0);
    }

    public function testThrowsWhenAuthorAccountIsEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('Author');
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

    private function buildWithReadyAtOverride(?CarbonImmutable $readyAt): PullRequest
    {
        $createdAt = CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');

        return new PullRequest(
            repoId: new RepoId(7),
            number: new PullRequestNumber(42),
            author: new Author('alice'),
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
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload(), repoId: 7);
        $this->assertSame(PullRequestStatus::Open, $pr->status);
        $this->assertFalse($pr->isDraft());
        $this->assertNull($pr->mergedAt);
    }

    public function testFromGitHubRawTranslatesMergedPr(): void
    {
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload([
            'state' => 'closed',
            'merged_at' => '2026-04-15T11:00:00Z',
            'closed_at' => '2026-04-15T11:00:00Z',
        ]), repoId: 7);
        $this->assertSame(PullRequestStatus::Merged, $pr->status);
        $this->assertNotNull($pr->mergedAt);
    }

    public function testFromGitHubRawTranslatesDraftPr(): void
    {
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload(['draft' => true]), repoId: 7);
        $this->assertTrue($pr->isDraft());
        $this->assertNull($pr->readyAt);
    }

    public function testFromGitHubRawThrowsWhenNumberMissing(): void
    {
        $payload = $this->payload();
        unset($payload['number']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('number');
        GitHubPullRequestFactory::fromGitHubRaw($payload, repoId: 7);
    }

    public function testFromGitHubRawParsesTimesAsUtc(): void
    {
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload([
            'created_at' => '2026-04-15T10:00:00Z',
        ]), repoId: 7);

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
    ): PullRequest {
        $createdAt ??= CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC');

        return new PullRequest(
            repoId: new RepoId(7),
            number: new PullRequestNumber($number),
            author: new Author($authorAccount),
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
