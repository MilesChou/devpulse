<?php

declare(strict_types=1);

namespace App\Infrastructure\Vcs\GitHub;

use DevPulse\Shared\MonthRange;
use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\Factory\GitHubPullRequestFactory;
use DevPulse\Vcs\PullRequest;
use DevPulse\Vcs\PullRequestId;
use DevPulse\Vcs\ReviewSummary;
use App\Support\Saloon\PayloadHelpers;
use Generator;
use InvalidArgumentException;
use Symfony\Component\Uid\Ulid;

class GitHubProvider
{
    public function __construct(private readonly GitHubConnector $connector)
    {
    }

    /**
     * 列出指定 repo 在指定月份內建立的所有 PR（已合併、已關閉、被 reject、仍在 draft 都包含）。
     *
     * @return Generator<int, PullRequest>
     */
    public function listPullRequestsInMonth(string $repoId, RepoFullName $repoFullName, MonthRange $month): Generator
    {
        $page = 1;
        $perPage = 100;

        while (true) {
            $response = $this->connector->send(new ListPullRequestsRequest((string)$repoFullName, $page, $perPage));
            $pulls = PayloadHelpers::listOfArrays($response->json());
            if ($pulls === []) {
                break;
            }

            $reachedOlderThanRange = false;
            foreach ($pulls as $rawPull) {
                $id = new PullRequestId((string)new Ulid());
                $pr = GitHubPullRequestFactory::fromGitHubRaw($rawPull, repoId: $repoId, id: $id);

                if ($pr->createdAt >= $month->end) {
                    continue;
                }
                if ($pr->createdAt < $month->start) {
                    $reachedOlderThanRange = true;
                    continue;
                }

                yield $pr;
            }

            if ($reachedOlderThanRange) {
                break;
            }
            if (count($pulls) < $perPage) {
                break;
            }
            $page++;
        }
    }

    /**
     * 取得單一 PR 的細節（含精確的 additions / deletions —— list endpoint 不回這兩個欄位）。
     */
    public function getPullRequest(string $repoId, RepoFullName $repoFullName, int $pullNumber): PullRequest
    {
        $response = $this->connector->send(new GetPullRequestRequest((string)$repoFullName, $pullNumber));
        $payload = PayloadHelpers::stringKeyedArray($response->json());

        $id = new PullRequestId((string)new Ulid());

        return GitHubPullRequestFactory::fromGitHubRaw($payload, repoId: $repoId, id: $id);
    }

    /**
     * 取得 commit 對應的 author GitHub login（找不到則回 null）。
     */
    public function getCommitAuthorAccount(RepoFullName $repoFullName, string $sha): ?string
    {
        $response = $this->connector->send(new GetCommitRequest((string)$repoFullName, $sha));
        $payload = PayloadHelpers::stringKeyedArray($response->json());

        $author = $payload['author'] ?? null;
        if (! is_array($author)) {
            return null;
        }

        $login = $author['login'] ?? null;

        return is_string($login) && $login !== '' ? $login : null;
    }

    /**
     * 一次處理一批 commit、回傳 [sha => author_login | null] 的對照表。
     *
     * 用 REST 一筆一打，量大時應改用 getCommitAuthorAccountsBulk。
     *
     * @param list<string> $shas
     * @return array<string, string|null>
     */
    public function getCommitAuthorAccounts(RepoFullName $repoFullName, array $shas): array
    {
        $result = [];
        foreach ($shas as $sha) {
            $result[$sha] = $this->getCommitAuthorAccount($repoFullName, $sha);
        }

        return $result;
    }

    /**
     * 用 GraphQL alias 一次撈最多 80 筆 commit author（仿 Python prototype）。
     *
     * 自動分批：sha 數量超過 80 時切成多次 GraphQL request。
     * 比 REST 一筆一打快約 80 倍。找不到對應 user.login 的 sha 對應 null。
     *
     * @param list<string> $shas
     * @return array<string, string|null>
     */
    public function getCommitAuthorAccountsBulk(RepoFullName $repoFullName, array $shas): array
    {
        $result = [];
        $batches = array_chunk($shas, GetCommitAuthorsBulkQuery::MAX_BATCH);

        foreach ($batches as $batch) {
            $response = $this->connector->send(new GetCommitAuthorsBulkQuery($repoFullName, $batch));
            $payload = PayloadHelpers::stringKeyedArray($response->json());
            $data = PayloadHelpers::stringKeyedArray($payload['data'] ?? null);
            $repository = PayloadHelpers::stringKeyedArray($data['repository'] ?? null);

            foreach ($batch as $i => $sha) {
                $node = PayloadHelpers::stringKeyedArray($repository["c{$i}"] ?? null);
                $author = PayloadHelpers::stringKeyedArray($node['author'] ?? null);
                $user = PayloadHelpers::stringKeyedArray($author['user'] ?? null);
                $login = $user['login'] ?? null;
                $result[$sha] = (is_string($login) && $login !== '') ? $login : null;
            }
        }

        return $result;
    }

    /**
     * 取得指定 PR 的所有 review（含 ready_at 用的精確時間戳，需要 GraphQL）。
     *
     * @return list<ReviewSummary>
     */
    public function listReviews(RepoFullName $repoFullName, int $pullNumber): array
    {
        $response = $this->connector->send(new GetPullRequestReviewsQuery($repoFullName, $pullNumber));
        $payload = PayloadHelpers::stringKeyedArray($response->json());

        $data = PayloadHelpers::stringKeyedArray($payload['data'] ?? null);
        $repository = PayloadHelpers::stringKeyedArray($data['repository'] ?? null);
        $pullRequest = PayloadHelpers::stringKeyedArray($repository['pullRequest'] ?? null);
        $reviews = PayloadHelpers::stringKeyedArray($pullRequest['reviews'] ?? null);
        $nodes = PayloadHelpers::listOfArrays($reviews['nodes'] ?? null);

        $result = [];
        foreach ($nodes as $node) {
            try {
                $result[] = ReviewSummary::fromGitHubGraphQL($node, $repoFullName, $pullNumber);
            } catch (InvalidArgumentException) {
                // PENDING review 的 submittedAt 為 null、ghost 帳號的 author.login 為 null
                // 這些是合法但不可用於 latency 計算的 node，跳過即可
                continue;
            }
        }

        return $result;
    }
}
