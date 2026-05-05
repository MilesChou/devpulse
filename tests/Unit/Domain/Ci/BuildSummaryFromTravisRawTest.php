<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Ci;

use App\Domain\Ci\BuildStatus;
use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\CiProviderType;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

class BuildSummaryFromTravisRawTest extends TestCase
{
    public function testTranslatesPushToMasterAsPostMerge(): void
    {
        $build = BuildSummary::fromTravisRaw($this->payload([
            'event_type' => 'push',
            'branch' => ['name' => 'master'],
        ]));

        $this->assertSame(CiProviderType::Travis, $build->provider);
        $this->assertTrue($build->isPostMerge());
        $this->assertFalse($build->isPullRequest());
    }

    public function testTranslatesPullRequestEvent(): void
    {
        $build = BuildSummary::fromTravisRaw($this->payload([
            'event_type' => 'pull_request',
            'branch' => ['name' => 'feature/foo'],
        ]));

        $this->assertTrue($build->isPullRequest());
    }

    public function testTranslatesCronAsDeployEvent(): void
    {
        $build = BuildSummary::fromTravisRaw($this->payload(['event_type' => 'cron']));
        $this->assertTrue($build->isDeployEvent());
    }

    public function testParsesIntegerIdAsString(): void
    {
        $build = BuildSummary::fromTravisRaw($this->payload(['id' => 123456]));
        $this->assertSame('123456', $build->externalId);
    }

    public function testParsesStartedAtToUtcCarbon(): void
    {
        $build = BuildSummary::fromTravisRaw($this->payload([
            'started_at' => '2026-04-15T10:00:00Z',
        ]));

        $this->assertSame('UTC', $build->startedAt->getTimezone()->getName());
        $this->assertSame('2026-04-15 10:00:00', $build->startedAt->format('Y-m-d H:i:s'));
    }

    public function testParsesPassedState(): void
    {
        $build = BuildSummary::fromTravisRaw($this->payload(['state' => 'passed']));
        $this->assertSame(BuildStatus::Passed, $build->status);
        $this->assertFalse($build->isFailure());
    }

    public function testParsesFailedState(): void
    {
        $build = BuildSummary::fromTravisRaw($this->payload(['state' => 'failed']));
        $this->assertTrue($build->isFailure());
    }

    public function testHandlesNullDuration(): void
    {
        $payload = $this->payload();
        unset($payload['duration']);

        $build = BuildSummary::fromTravisRaw($payload);
        $this->assertNull($build->durationSeconds);
    }

    public function testThrowsWhenRepositorySlugMissing(): void
    {
        $payload = $this->payload();
        unset($payload['repository']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('repository.slug');
        BuildSummary::fromTravisRaw($payload);
    }

    public function testThrowsWhenCommitShaMissing(): void
    {
        $payload = $this->payload();
        unset($payload['commit']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('commit.sha');
        BuildSummary::fromTravisRaw($payload);
    }

    public function testThrowsWhenStateMissing(): void
    {
        $payload = $this->payload();
        unset($payload['state']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('state');
        BuildSummary::fromTravisRaw($payload);
    }

    public function testThrowsWhenStartedAtMissing(): void
    {
        $payload = $this->payload();
        unset($payload['started_at']);

        $this->expectException(InvalidArgumentException::class);
        $this->expectExceptionMessage('started_at');
        BuildSummary::fromTravisRaw($payload);
    }

    /**
     * @param array<string, mixed> $overrides
     * @return array<string, mixed>
     */
    private function payload(array $overrides = []): array
    {
        return array_replace([
            'id' => 123456,
            'state' => 'passed',
            'event_type' => 'pull_request',
            'started_at' => '2026-04-15T10:00:00Z',
            'duration' => 120,
            'branch' => ['name' => 'feature/foo'],
            'commit' => ['sha' => 'abcdef0123456789'],
            'repository' => ['slug' => 'your-org/your-repo'],
        ], $overrides);
    }
}
