<?php

declare(strict_types=1);

namespace Tests\Feature\Domain\Ci\Travis;

use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\Travis\TravisConnector;
use App\Domain\Ci\Travis\TravisProvider;
use Saloon\Http\Faking\MockClient;
use Saloon\Http\Faking\MockResponse;
use Tests\TestCase;

class TravisProviderTest extends TestCase
{
    public function testListBuildsInMonthFiltersByMonthRange(): void
    {
        $mock = new MockClient([
            MockResponse::make([
                '@pagination' => ['next' => null],
                'builds' => [
                    $this->build(id: 1, startedAt: '2026-04-01T00:00:00Z'),  // 範圍內
                    $this->build(id: 2, startedAt: '2026-04-15T10:00:00Z'),  // 範圍內
                    $this->build(id: 3, startedAt: '2026-04-30T23:59:59Z'),  // 範圍內
                    $this->build(id: 4, startedAt: '2026-05-01T00:00:00Z'),  // 5 月：跳過
                    $this->build(id: 5, startedAt: '2026-03-31T23:00:00Z'),  // 3 月：早於範圍 → break
                ],
            ]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array($provider->listBuildsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(3, $builds);
        $this->assertSame(['1', '2', '3'], array_map(static fn (BuildSummary $b): string => $b->externalId, $builds));
    }

    public function testListBuildsInMonthFollowsPagination(): void
    {
        // page 1: 25 builds（滿頁，要繼續抓 page 2）
        $page1 = array_map(
            fn (int $i) => $this->build(id: $i, startedAt: sprintf('2026-04-%02dT10:00:00Z', max(1, 28 - $i))),
            range(1, 25),
        );
        // page 2: 5 builds（不滿頁，抓完即止）
        $page2 = array_map(
            fn (int $i) => $this->build(id: $i + 25, startedAt: sprintf('2026-04-%02dT10:00:00Z', max(1, 3 - ($i - 1)))),
            range(1, 5),
        );

        $mock = new MockClient([
            MockResponse::make(['builds' => $page1]),
            MockResponse::make(['builds' => $page2]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array($provider->listBuildsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(30, $builds);
    }

    public function testListBuildsInMonthStopsWhenReachingOlderThanRange(): void
    {
        $mock = new MockClient([
            MockResponse::make([
                'builds' => [
                    $this->build(id: 100, startedAt: '2026-04-15T10:00:00Z'),
                    $this->build(id: 99, startedAt: '2026-03-15T10:00:00Z'),  // 早於範圍 → 應 break
                ],
            ]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array($provider->listBuildsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(1, $builds);
        $this->assertSame('100', $builds[0]->externalId);
    }

    public function testGetBuildLogConcatenatesAllJobsLogs(): void
    {
        $mock = new MockClient([
            MockResponse::make(['jobs' => [['id' => 11], ['id' => 12]]]),
            MockResponse::make(['content' => 'job 11 log']),
            MockResponse::make(['content' => 'job 12 log']),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $log = $provider->getBuildLog('your-org/your-repo', '12345');

        $this->assertSame("job 11 log\njob 12 log", $log);
    }

    public function testGetBuildLogReturnsEmptyWhenNoJobs(): void
    {
        $mock = new MockClient([
            MockResponse::make(['jobs' => []]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $this->assertSame('', $provider->getBuildLog('your-org/your-repo', '12345'));
    }

    public function testRetriesOn5xxThenSucceeds(): void
    {
        $mock = new MockClient([
            MockResponse::make(['error' => 'down'], 503),
            MockResponse::make(['error' => 'down'], 502),
            MockResponse::make(['builds' => [$this->build(id: 1, startedAt: '2026-04-15T10:00:00Z')]]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array($provider->listBuildsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(1, $builds);
        $this->assertSame('1', $builds[0]->externalId);
    }

    public function testRetriesOnRateLimitThenSucceeds(): void
    {
        $mock = new MockClient([
            MockResponse::make(['error' => 'rate_limit'], 429),
            MockResponse::make(['builds' => [$this->build(id: 1, startedAt: '2026-04-15T10:00:00Z')]]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array($provider->listBuildsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(1, $builds);
    }

    public function testDoesNotRetryOn4xxClientError(): void
    {
        $mock = new MockClient([
            MockResponse::make(['error' => 'unauthorized'], 401),
        ]);

        $provider = new TravisProvider($this->connector($mock));

        $this->expectException(\Saloon\Exceptions\Request\RequestException::class);
        iterator_to_array($provider->listBuildsInMonth('your-org/your-repo', '2026-04'), false);
    }

    private function connector(MockClient $mock): TravisConnector
    {
        $connector = new TravisConnector('test-token');
        $connector->retryInterval = 0;  // 測試時關掉 sleep
        $connector->withMockClient($mock);

        return $connector;
    }

    /**
     * @return array<string, mixed>
     */
    private function build(int $id, string $startedAt, string $eventType = 'pull_request'): array
    {
        return [
            'id' => $id,
            'state' => 'passed',
            'event_type' => $eventType,
            'started_at' => $startedAt,
            'duration' => 120,
            'branch' => ['name' => 'feature/foo'],
            'commit' => ['sha' => 'abcdef0123456789'],
            'repository' => ['slug' => 'your-org/your-repo'],
        ];
    }
}
