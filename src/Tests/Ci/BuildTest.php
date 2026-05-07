<?php

declare(strict_types=1);

namespace Tests\Domain\Ci;

use DevPulse\Ci\Build;
use DevPulse\Ci\BuildStatus;
use DevPulse\Ci\BuildTrigger;
use DevPulse\Shared\CommitSha;
use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\PullRequestNumber;
use Carbon\CarbonImmutable;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class BuildTest extends TestCase
{
    public function testBuildsValidInstance(): void
    {
        $build = $this->build();
        $this->assertSame('12345', $build->externalId);
        $this->assertSame(BuildStatus::PASSED, $build->status);
    }

    public function testThrowsWhenExternalIdIsEmpty(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('externalId');
        $this->build(externalId: '');
    }

    public function testThrowsWhenCommitShaTooShort(): void
    {
        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('commitSha');
        $this->build(commitSha: new CommitSha('abc'));
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

    public function testIsPostMerge(): void
    {
        $build = $this->build(trigger: BuildTrigger::POST_MERGE);
        $this->assertTrue($build->isPostMerge());
        $this->assertFalse($build->isPullRequest());
        $this->assertFalse($build->isDeployEvent());
    }

    public function testIsPullRequest(): void
    {
        $build = $this->build(trigger: BuildTrigger::PULL_REQUEST);
        $this->assertTrue($build->isPullRequest());
        $this->assertFalse($build->isPostMerge());
        $this->assertFalse($build->isDeployEvent());
    }

    public function testIsDeployEventWhenScheduled(): void
    {
        $build = $this->build(trigger: BuildTrigger::SCHEDULED);
        $this->assertTrue($build->isDeployEvent());
        $this->assertFalse($build->isPostMerge());
        $this->assertFalse($build->isPullRequest());
    }

    public function testIsDeployEventWhenManual(): void
    {
        $build = $this->build(trigger: BuildTrigger::MANUAL);
        $this->assertTrue($build->isDeployEvent());
    }

    public function testIsFailureDelegatesToStatus(): void
    {
        $this->assertTrue($this->build(status: BuildStatus::FAILED)->isFailure());
        $this->assertFalse($this->build(status: BuildStatus::PASSED)->isFailure());
        $this->assertFalse($this->build(status: BuildStatus::CANCELED)->isFailure());
        $this->assertFalse($this->build(status: BuildStatus::IN_PROGRESS)->isFailure());
    }

    private function build(
        string $externalId = '12345',
        CommitSha $commitSha = new CommitSha('abcdef0'),
        ?string $authorAccount = null,
        ?int $prNumber = null,
        BuildStatus $status = BuildStatus::PASSED,
        BuildTrigger $trigger = BuildTrigger::PULL_REQUEST,
        ?string $branch = 'feature/foo',
        ?CarbonImmutable $startedAt = null,
        ?int $durationSeconds = 120,
    ): Build {
        return new Build(
            externalId: $externalId,
            repoFullName: new RepoFullName('your-org/your-repo'),
            commitSha: $commitSha,
            authorAccount: $authorAccount,
            prNumber: $prNumber !== null ? new PullRequestNumber($prNumber) : null,
            status: $status,
            trigger: $trigger,
            branch: $branch,
            startedAt: $startedAt ?? CarbonImmutable::parse('2026-04-15T10:00:00Z'),
            durationSeconds: $durationSeconds,
        );
    }
}
