<?php

declare(strict_types=1);

namespace Tests\Unit\Domain\Ci;

use App\Domain\Ci\BuildStatus;
use App\Domain\Ci\BuildTrigger;
use App\Domain\Shared\RepoFullName;
use App\Infrastructure\Ci\Travis\TravisConnector;
use App\Infrastructure\Ci\Travis\TravisProvider;
use Saloon\Http\Faking\MockClient;
use Saloon\Http\Faking\MockResponse;
use Tests\TestCase;

class BuildSummaryFromTravisRawTest extends TestCase
{
    public function testTranslatesPushToMasterAsPostMerge(): void
    {
        $build = $this->parse([
            'event_type' => 'push',
            'branch' => ['name' => 'master'],
        ]);

        $this->assertSame(BuildTrigger::POST_MERGE, $build->trigger);
        $this->assertTrue($build->isPostMerge());
        $this->assertFalse($build->isPullRequest());
    }

    public function testTranslatesPullRequestEvent(): void
    {
        $build = $this->parse([
            'event_type' => 'pull_request',
            'branch' => ['name' => 'feature/foo'],
        ]);

        $this->assertSame(BuildTrigger::PULL_REQUEST, $build->trigger);
        $this->assertTrue($build->isPullRequest());
    }

    public function testTranslatesCronAsScheduled(): void
    {
        $build = $this->parse(['event_type' => 'cron']);
        $this->assertSame(BuildTrigger::SCHEDULED, $build->trigger);
        $this->assertTrue($build->isDeployEvent());
    }

    public function testTranslatesApiAsManual(): void
    {
        $build = $this->parse(['event_type' => 'api']);
        $this->assertSame(BuildTrigger::MANUAL, $build->trigger);
        $this->assertTrue($build->isDeployEvent());
    }

    public function testParsesIntegerIdAsString(): void
    {
        $build = $this->parse(['id' => 123456]);
        $this->assertSame('123456', $build->externalId);
    }

    public function testParsesStartedAtToUtcCarbon(): void
    {
        $build = $this->parse(['started_at' => '2026-04-15T10:00:00Z']);

        $this->assertSame('UTC', $build->startedAt->getTimezone()->getName());
        $this->assertSame('2026-04-15 10:00:00', $build->startedAt->format('Y-m-d H:i:s'));
    }

    public function testParsesPassedState(): void
    {
        $build = $this->parse(['state' => 'passed']);
        $this->assertSame(BuildStatus::PASSED, $build->status);
        $this->assertFalse($build->isFailure());
    }

    public function testParsesFailedState(): void
    {
        $build = $this->parse(['state' => 'failed']);
        $this->assertTrue($build->isFailure());
    }

    public function testHandlesNullDuration(): void
    {
        $payload = $this->payload();
        unset($payload['duration']);

        $build = $this->parse($payload, raw: true);
        $this->assertNull($build->durationSeconds);
    }

    /**
     * @return iterable<string, array{string}>
     */
    public static function malformedPayloadFieldProvider(): iterable
    {
        yield 'missing repository' => ['repository'];
        yield 'missing commit' => ['commit'];
        yield 'missing state' => ['state'];
        yield 'missing started_at and finished_at' => ['started_at_and_finished_at'];
    }

    /**
     * 真實 Travis 回傳會混入欄位不全的 build（例如 cancelled-before-start）。
     * provider 應跳過這些壞 payload，繼續處理其他 build，而不是中斷整批。
     */
    #[\PHPUnit\Framework\Attributes\DataProvider('malformedPayloadFieldProvider')]
    public function testSkipsBuildWithMalformedPayload(string $missingField): void
    {
        $payload = $this->payload();
        if ($missingField === 'started_at_and_finished_at') {
            unset($payload['started_at']);
        } else {
            unset($payload[$missingField]);
        }

        $mock = new MockClient([MockResponse::make(['builds' => [$payload]])]);
        $connector = new TravisConnector('test-token');
        $connector->withMockClient($mock);

        $provider = new TravisProvider($connector);
        $builds = iterator_to_array(
            $provider->listBuildsInMonth(
                new RepoFullName('your-org/your-repo'),
                \App\Domain\Shared\MonthRange::fromString('2026-04'),
            ),
            false,
        );

        $this->assertSame([], $builds);
    }

    /**
     * @param array<string, mixed> $overrides
     */
    private function parse(array $overrides, bool $raw = false): \App\Domain\Ci\BuildSummary
    {
        $payload = $raw ? $overrides : array_replace($this->payload(), $overrides);

        $mock = new MockClient([
            MockResponse::make(['builds' => [$payload]]),
        ]);
        $connector = new TravisConnector('test-token');
        $connector->withMockClient($mock);

        $provider = new TravisProvider($connector);
        $builds = iterator_to_array(
            $provider->listBuildsInMonth(
                new RepoFullName('your-org/your-repo'),
                \App\Domain\Shared\MonthRange::fromString('2026-04'),
            ),
            false,
        );

        return $builds[0];
    }

    /**
     * @return array<string, mixed>
     */
    private function payload(): array
    {
        return [
            'id' => 123456,
            'state' => 'passed',
            'event_type' => 'pull_request',
            'started_at' => '2026-04-15T10:00:00Z',
            'duration' => 120,
            'branch' => ['name' => 'feature/foo'],
            'commit' => ['sha' => 'abcdef0123456789'],
            'repository' => ['slug' => 'your-org/your-repo'],
        ];
    }
}
