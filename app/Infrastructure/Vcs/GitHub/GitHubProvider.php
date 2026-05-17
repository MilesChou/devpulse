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
use JsonException;
use Saloon\Exceptions\Request\FatalRequestException;
use Saloon\Exceptions\Request\RequestException;
use Symfony\Component\Uid\Ulid;

class GitHubProvider
{
    public const string RATE_LIMITER = 'github';

    public function __construct(private readonly GitHubConnector $connector)
    {
    }

    /**
     * List all historical PRs for the given repo (state=all, newest first, no time cutoff).
     *
     * @return Generator<int, PullRequest>
     */
    public function listAllPullRequests(string $repoId, RepoFullName $repoFullName): Generator
    {
        yield from $this->paginatePullRequests($repoId, $repoFullName);
    }

    /**
     * List all PRs created within the given month (merged, closed, rejected, and draft all included).
     *
     * @return Generator<int, PullRequest>
     */
    public function listPullRequestsInMonth(string $repoId, RepoFullName $repoFullName, MonthRange $month): Generator
    {
        foreach ($this->paginatePullRequests($repoId, $repoFullName) as $pr) {
            if ($pr->createdAt >= $month->end) {
                continue;
            }
            if ($pr->createdAt < $month->start) {
                return;
            }

            yield $pr;
        }
    }

    /**
     * @return Generator<int, PullRequest>
     * @throws JsonException
     * @throws FatalRequestException
     * @throws RequestException
     */
    private function paginatePullRequests(
        string $repoId,
        RepoFullName $repoFullName,
        int $page = 1,
        int $perPage = 100,
    ): Generator {
        while (true) {
            $response = $this->connector->send(new ListPullRequestsRequest((string)$repoFullName, $page, $perPage));
            $pulls = PayloadHelpers::listOfArrays($response->json());
            if ($pulls === []) {
                return;
            }

            foreach ($pulls as $rawPull) {
                $id = new PullRequestId((string)new Ulid());
                yield GitHubPullRequestFactory::fromGitHubRaw($rawPull, repoId: $repoId, id: $id);
            }

            if (count($pulls) < $perPage) {
                return;
            }
            $page++;
        }
    }

    /**
     * Get details for a single PR (including accurate additions/deletions, which the list endpoint omits).
     */
    public function getPullRequest(string $repoId, RepoFullName $repoFullName, int $pullNumber): PullRequest
    {
        $response = $this->connector->send(new GetPullRequestRequest((string)$repoFullName, $pullNumber));
        $payload = PayloadHelpers::stringKeyedArray($response->json());

        $id = new PullRequestId((string)new Ulid());

        return GitHubPullRequestFactory::fromGitHubRaw($payload, repoId: $repoId, id: $id);
    }

    /**
     * Get the GitHub login of a commit's author. Returns null if not found.
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
     * Process a batch of commits and return a [sha => author_login|null] map.
     *
     * Uses one REST call per commit; prefer getCommitAuthorAccountsBulk for large batches.
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
     * Fetch up to 80 commit authors in one GraphQL request using aliases.
     *
     * Automatically batches when sha count exceeds 80.
     * ~80x faster than individual REST calls. Shas with no matching user.login map to null.
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
     * Get all reviews for the given PR. Uses GraphQL for accurate timestamps (needed for ready_at).
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
                // PENDING reviews have a null submittedAt; ghost accounts have a null author.login.
                // Both are valid but unusable for latency calculations — skip them.
                continue;
            }
        }

        return $result;
    }
}
