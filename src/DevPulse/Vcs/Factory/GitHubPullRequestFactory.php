<?php

declare(strict_types=1);

namespace DevPulse\Vcs\Factory;

use DateMalformedStringException;
use DevPulse\Shared\RepoId;
use DevPulse\Shared\UtcTimestamp;
use DevPulse\Vcs\Author;
use DevPulse\Vcs\ChangeStats;
use DevPulse\Vcs\Platform;
use DevPulse\Vcs\PullRequest;
use DevPulse\Vcs\PullRequestFactory;
use DevPulse\Vcs\PullRequestId;
use DevPulse\Vcs\PullRequestNumber;
use DevPulse\Vcs\PullRequestStatus;
use InvalidArgumentException;

final class GitHubPullRequestFactory implements PullRequestFactory
{
    /**
     * Build a PullRequest from a GitHub REST API PR payload.
     *
     * @param array<string, mixed> $raw
     * @throws DateMalformedStringException
     */
    public function fromRaw(array $raw, string $repoId, PullRequestId $id): PullRequest
    {
        $number = $raw['number'] ?? null;
        if (! is_int($number)) {
            throw new InvalidArgumentException('GitHub PR payload missing number');
        }

        $user = $raw['user'] ?? null;
        if (! is_array($user) || ! is_string($user['login'] ?? null)) {
            throw new InvalidArgumentException('GitHub PR payload missing user.login');
        }

        $additions = $raw['additions'] ?? null;
        $deletions = $raw['deletions'] ?? null;

        $createdAt = UtcTimestamp::required($raw, 'created_at', 'GitHub PR payload missing created_at');
        $isDraft = ($raw['draft'] ?? false) === true;

        return new PullRequest(
            id: $id,
            platform: Platform::GitHub,
            repoId: new RepoId($repoId),
            number: new PullRequestNumber($number),
            author: new Author($user['login']),
            status: $this->resolveStatus($raw),
            changes: new ChangeStats(
                additions: is_int($additions) ? $additions : 0,
                deletions: is_int($deletions) ? $deletions : 0,
            ),
            createdAt: $createdAt,
            readyAt: $isDraft ? null : $createdAt,
            closedAt: UtcTimestamp::optional($raw, 'closed_at'),
        );
    }

    /**
     * @param array<string, mixed> $raw
     */
    private function resolveStatus(array $raw): PullRequestStatus
    {
        $state = $raw['state'] ?? '';
        $mergedAt = $raw['merged_at'] ?? null;

        return PullRequestStatus::fromGitHubState(
            is_string($state) ? $state : '',
            is_string($mergedAt) ? $mergedAt : null,
        );
    }
}
