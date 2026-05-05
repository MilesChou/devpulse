<?php

declare(strict_types=1);

namespace App\Domain\Vcs\GitHub;

use App\Domain\Vcs\PullRequestSummary;
use App\Support\Saloon\PayloadHelpers;
use App\Support\Time\MonthRange;
use Generator;

class GitHubProvider
{
    public function __construct(private readonly GitHubConnector $connector)
    {
    }

    /**
     * 列出指定 repo 在指定月份內建立的所有 PR（已合併、已關閉、被 reject、仍在 draft 都包含）。
     *
     * 月份格式：YYYY-MM。
     *
     * @return Generator<int, PullRequestSummary>
     */
    public function listPullRequestsInMonth(string $repoFullName, string $month): Generator
    {
        [$start, $end] = MonthRange::parse($month);
        $page = 1;
        $perPage = 100;

        while (true) {
            $response = $this->connector->send(new ListPullRequestsRequest($repoFullName, $page, $perPage));
            $pulls = PayloadHelpers::listOfArrays($response->json());
            if ($pulls === []) {
                break;
            }

            $reachedOlderThanRange = false;
            foreach ($pulls as $rawPull) {
                $pr = PullRequestSummary::fromGitHubRaw($rawPull);

                if ($pr->createdAt->greaterThanOrEqualTo($end)) {
                    continue;
                }
                if ($pr->createdAt->lessThan($start)) {
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
    public function getPullRequest(string $repoFullName, int $pullNumber): PullRequestSummary
    {
        $response = $this->connector->send(new GetPullRequestRequest($repoFullName, $pullNumber));
        $payload = PayloadHelpers::stringKeyedArray($response->json());

        return PullRequestSummary::fromGitHubRaw($payload);
    }

    /**
     * 取得 commit 對應的 author GitHub login（找不到則回 null）。
     */
    public function getCommitAuthorAccount(string $repoFullName, string $sha): ?string
    {
        $response = $this->connector->send(new GetCommitRequest($repoFullName, $sha));
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
     * @param list<string> $shas
     * @return array<string, string|null>
     */
    public function getCommitAuthorAccounts(string $repoFullName, array $shas): array
    {
        $result = [];
        foreach ($shas as $sha) {
            $result[$sha] = $this->getCommitAuthorAccount($repoFullName, $sha);
        }

        return $result;
    }
}
