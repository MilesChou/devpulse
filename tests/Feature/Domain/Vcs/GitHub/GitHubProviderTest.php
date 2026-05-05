<?php

declare(strict_types=1);

namespace Tests\Feature\Domain\Vcs\GitHub;

use App\Domain\Vcs\GitHub\GitHubConnector;
use App\Domain\Vcs\GitHub\GitHubProvider;
use App\Domain\Vcs\PullRequestStatus;
use App\Domain\Vcs\PullRequestSummary;
use App\Domain\Vcs\ReviewState;
use Saloon\Http\Faking\MockClient;
use Saloon\Http\Faking\MockResponse;
use Tests\TestCase;

class GitHubProviderTest extends TestCase
{
    public function testListPullRequestsInMonthFiltersByMonthRange(): void
    {
        $mock = new MockClient([
            MockResponse::make([
                $this->pull(number: 100, createdAt: '2026-04-30T23:59:59Z'),
                $this->pull(number: 99, createdAt: '2026-04-15T10:00:00Z'),
                $this->pull(number: 98, createdAt: '2026-04-01T00:00:00Z'),
                $this->pull(number: 97, createdAt: '2026-03-31T23:00:00Z'),  // 早於範圍
                $this->pull(number: 101, createdAt: '2026-05-01T00:00:00Z'),  // 晚於範圍：跳過
            ]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $prs = iterator_to_array($provider->listPullRequestsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(3, $prs);
        $this->assertSame([100, 99, 98], array_map(static fn (PullRequestSummary $p): int => $p->number, $prs));
    }

    public function testListPullRequestsInMonthFollowsPagination(): void
    {
        $page1 = array_map(
            fn (int $n) => $this->pull(number: $n, createdAt: sprintf('2026-04-%02dT10:00:00Z', max(1, 28 - ($n - 100)))),
            range(100, 199),
        );
        $page2 = [
            $this->pull(number: 200, createdAt: '2026-04-02T10:00:00Z'),
            $this->pull(number: 201, createdAt: '2026-04-01T10:00:00Z'),
        ];

        $mock = new MockClient([
            MockResponse::make($page1),
            MockResponse::make($page2),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $prs = iterator_to_array($provider->listPullRequestsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(102, $prs);
    }

    public function testGetPullRequestReturnsDetailedSummary(): void
    {
        $mock = new MockClient([
            MockResponse::make($this->pull(number: 42, createdAt: '2026-04-15T10:00:00Z', additions: 100, deletions: 20)),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $pr = $provider->getPullRequest('your-org/your-repo', 42);

        $this->assertSame(42, $pr->number);
        $this->assertSame(100, $pr->additions);
        $this->assertSame(20, $pr->deletions);
        $this->assertSame(120, $pr->totalChangedLines());
    }

    public function testGetCommitAuthorAccountReturnsLogin(): void
    {
        $mock = new MockClient([
            MockResponse::make(['author' => ['login' => 'alice']]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $this->assertSame('alice', $provider->getCommitAuthorAccount('your-org/your-repo', 'abc1234'));
    }

    public function testGetCommitAuthorAccountReturnsNullWhenAuthorMissing(): void
    {
        $mock = new MockClient([
            MockResponse::make(['author' => null]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $this->assertNull($provider->getCommitAuthorAccount('your-org/your-repo', 'abc1234'));
    }

    public function testGetCommitAuthorAccountsReturnsMap(): void
    {
        $mock = new MockClient([
            MockResponse::make(['author' => ['login' => 'alice']]),
            MockResponse::make(['author' => ['login' => 'bob']]),
            MockResponse::make(['author' => null]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $accounts = $provider->getCommitAuthorAccounts('your-org/your-repo', ['sha1', 'sha2', 'sha3']);

        $this->assertSame(['sha1' => 'alice', 'sha2' => 'bob', 'sha3' => null], $accounts);
    }

    public function testListReviewsParsesGraphQLResponse(): void
    {
        $mock = new MockClient([
            MockResponse::make([
                'data' => [
                    'repository' => [
                        'pullRequest' => [
                            'reviews' => [
                                'nodes' => [
                                    [
                                        'state' => 'APPROVED',
                                        'submittedAt' => '2026-04-15T11:00:00Z',
                                        'author' => ['login' => 'reviewer1'],
                                    ],
                                    [
                                        'state' => 'CHANGES_REQUESTED',
                                        'submittedAt' => '2026-04-15T12:00:00Z',
                                        'author' => ['login' => 'reviewer2'],
                                    ],
                                ],
                            ],
                        ],
                    ],
                ],
            ]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $reviews = $provider->listReviews('your-org/your-repo', 42);

        $this->assertCount(2, $reviews);
        $this->assertSame(ReviewState::Approved, $reviews[0]->state);
        $this->assertSame('reviewer1', $reviews[0]->reviewerAccount);
        $this->assertSame(ReviewState::ChangesRequested, $reviews[1]->state);
    }

    public function testListReviewsReturnsEmptyWhenNoReviews(): void
    {
        $mock = new MockClient([
            MockResponse::make([
                'data' => [
                    'repository' => [
                        'pullRequest' => [
                            'reviews' => ['nodes' => []],
                        ],
                    ],
                ],
            ]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $this->assertSame([], $provider->listReviews('your-org/your-repo', 42));
    }

    public function testRetriesOn5xxThenSucceeds(): void
    {
        $mock = new MockClient([
            MockResponse::make(['error' => 'down'], 503),
            MockResponse::make([$this->pull(number: 1, createdAt: '2026-04-15T10:00:00Z')]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $prs = iterator_to_array($provider->listPullRequestsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(1, $prs);
    }

    public function testRetriesOnRateLimitThenSucceeds(): void
    {
        $mock = new MockClient([
            MockResponse::make(['message' => 'rate limit'], 429),
            MockResponse::make([$this->pull(number: 1, createdAt: '2026-04-15T10:00:00Z')]),
        ]);

        $provider = new GitHubProvider($this->connector($mock));
        $prs = iterator_to_array($provider->listPullRequestsInMonth('your-org/your-repo', '2026-04'), false);

        $this->assertCount(1, $prs);
    }

    public function testDoesNotRetryOn4xxClientError(): void
    {
        $mock = new MockClient([
            MockResponse::make(['message' => 'unauthorized'], 401),
        ]);

        $provider = new GitHubProvider($this->connector($mock));

        $this->expectException(\Saloon\Exceptions\Request\RequestException::class);
        iterator_to_array($provider->listPullRequestsInMonth('your-org/your-repo', '2026-04'), false);
    }

    private function connector(MockClient $mock): GitHubConnector
    {
        $connector = new GitHubConnector('test-token');
        $connector->retryInterval = 0;
        $connector->withMockClient($mock);

        return $connector;
    }

    /**
     * @return array<string, mixed>
     */
    private function pull(
        int $number,
        string $createdAt,
        int $additions = 10,
        int $deletions = 5,
        string $login = 'alice',
        bool $draft = false,
    ): array {
        return [
            'number' => $number,
            'state' => 'open',
            'draft' => $draft,
            'additions' => $additions,
            'deletions' => $deletions,
            'created_at' => $createdAt,
            'merged_at' => null,
            'closed_at' => null,
            'user' => ['login' => $login],
            'base' => ['repo' => ['full_name' => 'your-org/your-repo']],
        ];
    }
}
