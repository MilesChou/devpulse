<?php

declare(strict_types=1);

namespace Tests\Feature\Infrastructure\Ci\Travis;

use App\Domain\Ci\Build;
use App\Domain\Shared\MonthRange;
use App\Domain\Shared\RepoFullName;
use App\Infrastructure\Ci\Travis\TravisConnector;
use App\Infrastructure\Ci\Travis\TravisProvider;
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
        $builds = iterator_to_array($provider->listBuildsInMonth(new RepoFullName('your-org/your-repo'), MonthRange::fromString('2026-04')), false);

        $this->assertCount(3, $builds);
        $this->assertSame(['1', '2', '3'], array_map(static fn (Build $b): string => $b->externalId, $builds));
    }

    public function testListBuildsInMonthFollowsPagination(): void
    {
        // page 1: 100 builds（滿頁，要繼續抓 page 2）
        $page1 = array_map(
            fn (int $i) => $this->build(id: $i, startedAt: sprintf('2026-04-%02dT10:00:00Z', (($i - 1) % 28) + 1)),
            range(1, 100),
        );
        // page 2: 5 builds（不滿頁，抓完即止）
        $page2 = array_map(
            fn (int $i) => $this->build(id: $i + 100, startedAt: sprintf('2026-04-%02dT10:00:00Z', max(1, 5 - ($i - 1)))),
            range(1, 5),
        );

        $mock = new MockClient([
            MockResponse::make(['builds' => $page1]),
            MockResponse::make(['builds' => $page2]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array(
            $provider->listBuildsInMonth(new RepoFullName('your-org/your-repo'), MonthRange::fromString('2026-04')),
            false,
        );

        $this->assertCount(105, $builds);
    }

    public function testListBuildsInMonthDoesNotStopOnSingleOldBuildWhenTimeAndIdAreInterleaved(): void
    {
        // Travis API 的 sort_by=id:desc 不嚴格等於時間倒序：
        // 例如 cancelled / re-run / push vs pull_request build 的 id 與 finished_at 會交錯。
        // 真實案例：page 內前 N 筆是早期年份的 cancelled，最後幾筆才是當月。
        // 不該因為遇到一筆 2018 build 就直接 break，會錯過後面的 2026-04 build。
        $mock = new MockClient([
            MockResponse::make([
                'builds' => [
                    $this->build(id: 100, startedAt: '2018-10-15T10:00:00Z'),  // 早於範圍但 id 大
                    $this->build(id: 99, startedAt: '2026-04-15T10:00:00Z'),   // 月內，但 id 較小
                    $this->build(id: 98, startedAt: '2026-04-10T10:00:00Z'),
                ],
            ]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array(
            $provider->listBuildsInMonth(new RepoFullName('your-org/your-repo'), MonthRange::fromString('2026-04')),
            false,
        );

        $this->assertCount(2, $builds);
        $this->assertSame(['99', '98'], array_map(static fn (Build $b): string => $b->externalId, $builds));
    }

    public function testListBuildsInMonthStopsAfter50ConsecutiveOlderBuilds(): void
    {
        // 累積 50 筆連續早於月份的 build 後才停（仿 Python prototype 行為）。
        // 用 49 筆早於 + 1 筆月內 + 50 筆早於：第二批 50 筆觸發 stop，但月內那筆已抓到。
        $builds = [];
        for ($i = 1; $i <= 49; $i++) {
            $builds[] = $this->build(id: 200 - $i, startedAt: '2018-01-01T00:00:00Z');
        }
        $builds[] = $this->build(id: 150, startedAt: '2026-04-15T10:00:00Z');
        for ($i = 1; $i <= 60; $i++) {
            $builds[] = $this->build(id: 100 - $i, startedAt: '2018-01-01T00:00:00Z');
        }

        $mock = new MockClient([MockResponse::make(['builds' => $builds])]);

        $provider = new TravisProvider($this->connector($mock));
        $result = iterator_to_array(
            $provider->listBuildsInMonth(new RepoFullName('your-org/your-repo'), MonthRange::fromString('2026-04')),
            false,
        );

        $this->assertCount(1, $result);
        $this->assertSame('150', $result[0]->externalId);
    }

    public function testGetBuildLogConcatenatesAllJobsLogs(): void
    {
        $mock = new MockClient([
            MockResponse::make(['jobs' => [['id' => 11], ['id' => 12]]]),
            MockResponse::make(['content' => 'job 11 log']),
            MockResponse::make(['content' => 'job 12 log']),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $log = $provider->getBuildLog(new RepoFullName('your-org/your-repo'), '12345');

        $this->assertSame("job 11 log\njob 12 log", $log);
    }

    public function testGetBuildLogReturnsEmptyWhenNoJobs(): void
    {
        $mock = new MockClient([
            MockResponse::make(['jobs' => []]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $this->assertSame('', $provider->getBuildLog(new RepoFullName('your-org/your-repo'), '12345'));
    }

    public function testRetriesOn5xxThenSucceeds(): void
    {
        $mock = new MockClient([
            MockResponse::make(['error' => 'down'], 503),
            MockResponse::make(['error' => 'down'], 502),
            MockResponse::make(['builds' => [$this->build(id: 1, startedAt: '2026-04-15T10:00:00Z')]]),
        ]);

        $provider = new TravisProvider($this->connector($mock));
        $builds = iterator_to_array($provider->listBuildsInMonth(new RepoFullName('your-org/your-repo'), MonthRange::fromString('2026-04')), false);

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
        $builds = iterator_to_array($provider->listBuildsInMonth(new RepoFullName('your-org/your-repo'), MonthRange::fromString('2026-04')), false);

        $this->assertCount(1, $builds);
    }

    public function testDoesNotRetryOn4xxClientError(): void
    {
        $mock = new MockClient([
            MockResponse::make(['error' => 'unauthorized'], 401),
        ]);

        $provider = new TravisProvider($this->connector($mock));

        $this->expectException(\Saloon\Exceptions\Request\RequestException::class);
        iterator_to_array($provider->listBuildsInMonth(new RepoFullName('your-org/your-repo'), MonthRange::fromString('2026-04')), false);
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
