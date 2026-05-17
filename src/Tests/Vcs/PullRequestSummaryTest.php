<?php

declare(strict_types=1);

namespace Tests\Domain\Vcs;

use DevPulse\Shared\RepoId;
use DevPulse\Vcs\Author;
use DevPulse\Vcs\ChangeStats;
use DevPulse\Vcs\Factory\GitHubPullRequestFactory;
use DevPulse\Vcs\Platform;
use DevPulse\Vcs\PullRequest;
use DevPulse\Vcs\PullRequestId;
use DevPulse\Vcs\PullRequestNumber;
use DevPulse\Vcs\PullRequestStatus;
use DateTimeImmutable;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class PullRequestSummaryTest extends TestCase
{
    public function testBuildsValidInstance(): void
    {
        $pr = $this->build();
        $this->assertSame('01JTESTREP00000000000000A7', $pr->repoId->toString());
        $this->assertSame(42, $pr->number->toInt());
        $this->assertSame(PullRequestStatus::Open, $pr->status());
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

    public function testThrowsWhenMergedWithoutClosedAt(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('closedAt');
        $this->build(status: PullRequestStatus::Merged, closedAt: null);
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

    private function buildWithReadyAtOverride(?DateTimeImmutable $readyAt): PullRequest
    {
        $createdAt = new DateTimeImmutable('2026-04-15T10:00:00Z');

        return new PullRequest(
            id: new PullRequestId('01JTEST000000000000000000A'),
            platform: Platform::GitHub,
            repoId: new RepoId('01JTESTREP00000000000000A7'),
            number: new PullRequestNumber(42),
            author: new Author('alice'),
            status: PullRequestStatus::Open,
            changes: new ChangeStats(10, 5),
            createdAt: $createdAt,
            readyAt: $readyAt,
            closedAt: null,
        );
    }

    public function testTotalChangedLines(): void
    {
        $this->assertSame(150, $this->build(additions: 100, deletions: 50)->totalChangedLines());
    }

    public function testFromGitHubRawTranslatesOpenPr(): void
    {
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload(), repoId: '01JTESTREP00000000000000A7', id: new PullRequestId('01JTEST000000000000000000F'));
        $this->assertSame(PullRequestStatus::Open, $pr->status());
        $this->assertFalse($pr->isDraft());
        $this->assertNull($pr->closedAt());
    }

    public function testFromGitHubRawTranslatesMergedPr(): void
    {
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload([
            'state' => 'closed',
            'merged_at' => '2026-04-15T11:00:00Z',
            'closed_at' => '2026-04-15T11:00:00Z',
        ]), repoId: '01JTESTREP00000000000000A7', id: new PullRequestId('01JTEST000000000000000000G'));
        $this->assertSame(PullRequestStatus::Merged, $pr->status());
        $this->assertNotNull($pr->closedAt());
    }

    public function testFromGitHubRawTranslatesDraftPr(): void
    {
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload(['draft' => true]), repoId: '01JTESTREP00000000000000A7', id: new PullRequestId('01JTEST000000000000000000H'));
        $this->assertTrue($pr->isDraft());
        $this->assertNull($pr->readyAt());
    }

    public function testFromGitHubRawThrowsWhenNumberMissing(): void
    {
        $payload = $this->payload();
        unset($payload['number']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('number');
        GitHubPullRequestFactory::fromGitHubRaw($payload, repoId: '01JTESTREP00000000000000A7', id: new PullRequestId('01JTEST000000000000000000I'));
    }

    public function testFromGitHubRawParsesTimesAsUtc(): void
    {
        $pr = GitHubPullRequestFactory::fromGitHubRaw($this->payload([
            'created_at' => '2026-04-15T10:00:00Z',
        ]), repoId: '01JTESTREP00000000000000A7', id: new PullRequestId('01JTEST000000000000000000J'));

        $this->assertSame('UTC', $pr->createdAt->getTimezone()->getName());
        $this->assertSame('2026-04-15 10:00:00', $pr->createdAt->format('Y-m-d H:i:s'));
    }

    private function build(
        int $number = 42,
        string $authorAccount = 'alice',
        PullRequestStatus $status = PullRequestStatus::Open,
        int $additions = 10,
        int $deletions = 5,
        ?DateTimeImmutable $createdAt = null,
        ?DateTimeImmutable $readyAt = null,
        ?DateTimeImmutable $closedAt = null,
    ): PullRequest {
        $createdAt ??= new DateTimeImmutable('2026-04-15T10:00:00Z');

        return new PullRequest(
            id: new PullRequestId('01JTEST000000000000000000B'),
            platform: Platform::GitHub,
            repoId: new RepoId('01JTESTREP00000000000000A7'),
            number: new PullRequestNumber($number),
            author: new Author($authorAccount),
            status: $status,
            changes: new ChangeStats($additions, $deletions),
            createdAt: $createdAt,
            readyAt: $readyAt ?? $createdAt,
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
