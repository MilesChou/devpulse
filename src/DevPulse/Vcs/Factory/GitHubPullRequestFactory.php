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
use DevPulse\Vcs\PullRequestId;
use DevPulse\Vcs\PullRequestNumber;
use DevPulse\Vcs\PullRequestStatus;
use InvalidArgumentException;

final class GitHubPullRequestFactory
{
    /**
     * 從 GitHub REST API 的 PR payload 建立 PullRequest。
     *
     * @param array<string, mixed> $raw
     */
    public static function fromGitHubRaw(array $raw, string $repoId, PullRequestId $id): PullRequest
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

        $status = self::resolveStatus($raw);

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
            createdAt: self::parseTimeRequired($raw, 'created_at'),
            readyAt: self::inferReadyAtFromDraft($raw),
            closedAt: self::parseTimeOptional($raw, 'closed_at'),
        );
    }

    /**
     * @param array<string, mixed> $raw
     */
    private static function resolveStatus(array $raw): PullRequestStatus
    {
        $state = $raw['state'] ?? '';
        $mergedAt = $raw['merged_at'] ?? null;

        return PullRequestStatus::fromGitHubState(
            is_string($state) ? $state : '',
            is_string($mergedAt) ? $mergedAt : null,
        );
    }

    /**
     * draft=true 時回傳 null（PR 尚未 ready）；否則以 created_at 作為 ready 時間。
     * GitHub 的 REST API 目前不提供「何時轉為 ready」的時間戳，以此近似取代。
     *
     * @param array<string, mixed> $raw
     */
    private static function inferReadyAtFromDraft(array $raw): ?DateTimeImmutable
    {
        $isDraft = $raw['draft'] ?? false;
        if ($isDraft === true) {
            return null;
        }

        return self::parseTimeRequired($raw, 'created_at');
    }

    /**
     * @param array<string, mixed> $raw
     */
    private static function parseTimeRequired(array $raw, string $key): DateTimeImmutable
    {
        $value = $raw[$key] ?? null;
        if (! is_string($value) || $value === '') {
            throw new InvalidArgumentException("GitHub PR payload missing {$key}");
        }

        return (new DateTimeImmutable($value))->setTimezone(new DateTimeZone('UTC'));
    }

    /**
     * @param array<string, mixed> $raw
     */
    private static function parseTimeOptional(array $raw, string $key): ?DateTimeImmutable
    {
        $value = $raw[$key] ?? null;
        if (! is_string($value) || $value === '') {
            return null;
        }

        return (new DateTimeImmutable($value))->setTimezone(new DateTimeZone('UTC'));
    }
}
