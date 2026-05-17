<?php

declare(strict_types=1);

namespace DevPulse\Vcs\Factory;

use DateTimeImmutable;
use DateTimeZone;
use DevPulse\Shared\RepoId;
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
     */
    public function fromRaw(array $raw, string $repoId, PullRequestId $id): PullRequest
    {
        $repoIdVo = new RepoId($repoId);
        $number = $raw['number'] ?? null;
        if (! is_int($number)) {
            throw new InvalidArgumentException('GitHub PR payload missing number');
        }
        $numberVo = new PullRequestNumber($number);

        $user = $raw['user'] ?? null;
        if (! is_array($user) || ! is_string($user['login'] ?? null)) {
            throw new InvalidArgumentException('GitHub PR payload missing user.login');
        }
        $authorVo = new Author($user['login']);

        $status = $this->resolveStatus($raw);

        $additions = $raw['additions'] ?? null;
        $deletions = $raw['deletions'] ?? null;

        return new PullRequest(
            id: $id,
            platform: Platform::GitHub,
            repoId: $repoIdVo,
            number: $numberVo,
            author: $authorVo,
            status: $status,
            changes: new ChangeStats(
                additions: is_int($additions) ? $additions : 0,
                deletions: is_int($deletions) ? $deletions : 0,
            ),
            createdAt: $this->parseCreatedAt($raw),
            readyAt: $this->inferReadyAtFromDraft($raw),
            closedAt: $this->parseClosedAt($raw),
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

    /**
     * Returns null when draft=true (PR not yet ready); otherwise falls back to created_at as the ready time.
     * GitHub's REST API does not expose the "marked-ready" timestamp, so this is the best approximation.
     *
     * @param array<string, mixed> $raw
     */
    private function inferReadyAtFromDraft(array $raw): ?DateTimeImmutable
    {
        $isDraft = $raw['draft'] ?? false;
        if ($isDraft === true) {
            return null;
        }

        return $this->parseCreatedAt($raw);
    }

    /**
     * GitHub timestamps are already UTC ("...Z"); setTimezone(UTC) only normalizes the
     * timezone object's name (e.g. "Z" or "+00:00" → "UTC"), it does NOT shift the instant.
     *
     * @param array<string, mixed> $raw
     */
    private function parseCreatedAt(array $raw): DateTimeImmutable
    {
        $value = $raw['created_at'] ?? null;
        if (! is_string($value) || $value === '') {
            throw new InvalidArgumentException('GitHub PR payload missing created_at');
        }

        return (new DateTimeImmutable($value))->setTimezone(new DateTimeZone('UTC'));
    }

    /**
     * See parseCreatedAt() for the setTimezone(UTC) rationale.
     *
     * @param array<string, mixed> $raw
     */
    private function parseClosedAt(array $raw): ?DateTimeImmutable
    {
        $value = $raw['closed_at'] ?? null;
        if (! is_string($value) || $value === '') {
            return null;
        }

        return (new DateTimeImmutable($value))->setTimezone(new DateTimeZone('UTC'));
    }
}
