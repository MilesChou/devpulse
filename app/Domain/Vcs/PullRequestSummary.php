<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use App\Domain\Shared\RepoFullName;
use Carbon\CarbonImmutable;
use InvalidArgumentException;

final readonly class PullRequestSummary
{
    public function __construct(
        public RepoFullName $repoFullName,
        public int $number,
        public string $authorAccount,
        public PullRequestStatus $status,
        public int $additions,
        public int $deletions,
        public CarbonImmutable $createdAt,
        public ?CarbonImmutable $readyAt,
        public ?CarbonImmutable $mergedAt,
        public ?CarbonImmutable $closedAt,
    ) {
        if ($number < 1) {
            throw new InvalidArgumentException('number must be >= 1');
        }
        if ($authorAccount === '') {
            throw new InvalidArgumentException('authorAccount must not be empty');
        }
        if ($additions < 0) {
            throw new InvalidArgumentException('additions must not be negative');
        }
        if ($deletions < 0) {
            throw new InvalidArgumentException('deletions must not be negative');
        }
        if ($status->isMerged() && $mergedAt === null) {
            throw new InvalidArgumentException('mergedAt must be set when status is merged');
        }
        if (! $status->isOpen() && $closedAt === null) {
            throw new InvalidArgumentException('closedAt must be set when status is closed or merged');
        }
    }

    public function isDraft(): bool
    {
        return $this->readyAt === null;
    }

    public function totalChangedLines(): int
    {
        return $this->additions + $this->deletions;
    }

    /**
     * 從 GitHub REST API 的 PR payload 建立 PullRequestSummary。
     *
     * @param array<string, mixed> $raw
     */
    public static function fromGitHubRaw(array $raw): self
    {
        $base = $raw['base'] ?? null;
        $repo = is_array($base) ? ($base['repo'] ?? null) : null;
        if (! is_array($repo) || ! is_string($repo['full_name'] ?? null)) {
            throw new InvalidArgumentException('GitHub PR payload missing base.repo.full_name');
        }
        $repoFullName = new RepoFullName($repo['full_name']);

        $number = $raw['number'] ?? null;
        if (! is_int($number)) {
            throw new InvalidArgumentException('GitHub PR payload missing number');
        }

        $user = $raw['user'] ?? null;
        if (! is_array($user) || ! is_string($user['login'] ?? null)) {
            throw new InvalidArgumentException('GitHub PR payload missing user.login');
        }

        $status = self::resolveStatus($raw);

        $additions = $raw['additions'] ?? null;
        $deletions = $raw['deletions'] ?? null;

        return new self(
            repoFullName: $repoFullName,
            number: $number,
            authorAccount: $user['login'],
            status: $status,
            additions: is_int($additions) ? $additions : 0,
            deletions: is_int($deletions) ? $deletions : 0,
            createdAt: self::parseTimeRequired($raw, 'created_at'),
            readyAt: self::inferReadyAtFromDraft($raw),
            mergedAt: self::parseTimeOptional($raw, 'merged_at'),
            closedAt: self::parseTimeOptional($raw, 'closed_at'),
        );
    }

    /**
     * @param array<string, mixed> $raw
     */
    private static function resolveStatus(array $raw): PullRequestStatus
    {
        if ($raw['merged_at'] ?? null) {
            return PullRequestStatus::Merged;
        }

        if (($raw['state'] ?? null) === PullRequestStatus::Closed->value) {
            return PullRequestStatus::Closed;
        }

        return PullRequestStatus::Open;
    }

    /**
     * @param array<string, mixed> $raw
     */
    private static function inferReadyAtFromDraft(array $raw): ?CarbonImmutable
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
    private static function parseTimeRequired(array $raw, string $key): CarbonImmutable
    {
        $value = $raw[$key] ?? null;
        if (! is_string($value) || $value === '') {
            throw new InvalidArgumentException("GitHub PR payload missing {$key}");
        }

        return CarbonImmutable::parse($value)->utc();
    }

    /**
     * @param array<string, mixed> $raw
     */
    private static function parseTimeOptional(array $raw, string $key): ?CarbonImmutable
    {
        $value = $raw[$key] ?? null;
        if (! is_string($value) || $value === '') {
            return null;
        }

        return CarbonImmutable::parse($value)->utc();
    }
}
