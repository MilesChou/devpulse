<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Ci;

use App\Domain\Ci\BuildStatus;
use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\CiProviderType;
use Carbon\CarbonImmutable;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class BuildSummaryTest extends TestCase
{
    public function testBuildsValidInstance(): void
    {
        $build = $this->build();
        $this->assertSame('12345', $build->externalId);
        $this->assertSame(BuildStatus::Passed, $build->status);
    }

    public function testThrowsWhenExternalIdIsEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('externalId');
        $this->build(externalId: '');
    }

    public function testThrowsWhenRepoFullNameMissingSlash(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('repoFullName');
        $this->build(repoFullName: 'invalid');
    }

    public function testThrowsWhenCommitShaTooShort(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('commitSha');
        $this->build(commitSha: 'abc');
    }

    public function testThrowsWhenEventTypeIsEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('eventType');
        $this->build(eventType: '');
    }

    public function testThrowsWhenDurationIsNegative(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('durationSeconds');
        $this->build(durationSeconds: -1);
    }

    public function testAcceptsNullDuration(): void
    {
        $build = $this->build(durationSeconds: null);
        $this->assertNull($build->durationSeconds);
    }

    public function testIsPostMergeWhenPushToMaster(): void
    {
        $build = $this->build(eventType: 'push', branch: 'master');
        $this->assertTrue($build->isPostMerge());
        $this->assertFalse($build->isPullRequest());
        $this->assertFalse($build->isDeployEvent());
    }

    public function testIsPostMergeWhenPushToMain(): void
    {
        $build = $this->build(eventType: 'push', branch: 'main');
        $this->assertTrue($build->isPostMerge());
    }

    public function testIsNotPostMergeWhenPushToFeatureBranch(): void
    {
        $build = $this->build(eventType: 'push', branch: 'feature/foo');
        $this->assertFalse($build->isPostMerge());
    }

    public function testIsNotPostMergeWhenBranchIsNull(): void
    {
        $build = $this->build(eventType: 'push', branch: null);
        $this->assertFalse($build->isPostMerge());
    }

    public function testIsPullRequestWhenEventTypeIsPullRequest(): void
    {
        $build = $this->build(eventType: 'pull_request');
        $this->assertTrue($build->isPullRequest());
        $this->assertFalse($build->isPostMerge());
        $this->assertFalse($build->isDeployEvent());
    }

    public function testIsDeployEventWhenEventTypeIsCron(): void
    {
        $build = $this->build(eventType: 'cron');
        $this->assertTrue($build->isDeployEvent());
        $this->assertFalse($build->isPostMerge());
        $this->assertFalse($build->isPullRequest());
    }

    public function testIsDeployEventWhenEventTypeIsApi(): void
    {
        $build = $this->build(eventType: 'api');
        $this->assertTrue($build->isDeployEvent());
    }

    public function testIsFailureDelegatesToStatus(): void
    {
        $this->assertTrue($this->build(status: BuildStatus::Failed)->isFailure());
        $this->assertTrue($this->build(status: BuildStatus::Errored)->isFailure());
        $this->assertFalse($this->build(status: BuildStatus::Passed)->isFailure());
        $this->assertFalse($this->build(status: BuildStatus::Canceled)->isFailure());
    }

    private function build(
        CiProviderType $provider = CiProviderType::Travis,
        string $externalId = '12345',
        string $repoFullName = 'your-org/your-repo',
        string $commitSha = 'abcdef0',
        ?string $authorAccount = null,
        ?int $prNumber = null,
        BuildStatus $status = BuildStatus::Passed,
        string $eventType = 'pull_request',
        ?string $branch = 'feature/foo',
        ?CarbonImmutable $startedAt = null,
        ?int $durationSeconds = 120,
    ): BuildSummary {
        return new BuildSummary(
            provider: $provider,
            externalId: $externalId,
            repoFullName: $repoFullName,
            commitSha: $commitSha,
            authorAccount: $authorAccount,
            prNumber: $prNumber,
            status: $status,
            eventType: $eventType,
            branch: $branch,
            startedAt: $startedAt ?? CarbonImmutable::create(2026, 4, 15, 10, 0, 0, 'UTC'),
            durationSeconds: $durationSeconds,
        );
    }
}
