<?php

declare(strict_types=1);

namespace App\Domain\Ci;

use Carbon\CarbonImmutable;
use InvalidArgumentException;

final readonly class BuildSummary
{
    private const TRUNK_BRANCHES = ['master', 'main'];

    private const DEPLOY_EVENT_TYPES = ['cron', 'api'];

    public function __construct(
        public CiProviderType $provider,
        public string $externalId,
        public string $repoFullName,
        public string $commitSha,
        public BuildStatus $status,
        public string $eventType,
        public ?string $branch,
        public CarbonImmutable $startedAt,
        public ?int $durationSeconds,
    ) {
        if ($externalId === '') {
            throw new InvalidArgumentException('externalId 不能是空字串');
        }
        if (! str_contains($repoFullName, '/')) {
            throw new InvalidArgumentException('repoFullName 必須是 owner/name 格式');
        }
        if (strlen($commitSha) < 7) {
            throw new InvalidArgumentException('commitSha 至少 7 字元');
        }
        if ($eventType === '') {
            throw new InvalidArgumentException('eventType 不能是空字串');
        }
        if ($durationSeconds !== null && $durationSeconds < 0) {
            throw new InvalidArgumentException('durationSeconds 不能為負');
        }
    }

    public function isPostMerge(): bool
    {
        return $this->eventType === 'push'
            && $this->branch !== null
            && in_array($this->branch, self::TRUNK_BRANCHES, true);
    }

    public function isPullRequest(): bool
    {
        return $this->eventType === 'pull_request';
    }

    public function isDeployEvent(): bool
    {
        return in_array($this->eventType, self::DEPLOY_EVENT_TYPES, true);
    }

    public function isFailure(): bool
    {
        return $this->status->isFailure();
    }

    /**
     * 從 Travis API 的 build payload 建立 BuildSummary。
     *
     * @param array<string, mixed> $raw
     */
    public static function fromTravisRaw(array $raw): self
    {
        $repository = $raw['repository'] ?? null;
        $commit = $raw['commit'] ?? null;
        $branch = $raw['branch'] ?? null;

        if (! is_array($repository) || ! is_string($repository['slug'] ?? null)) {
            throw new InvalidArgumentException('Travis payload 缺少 repository.slug');
        }
        if (! is_array($commit) || ! is_string($commit['sha'] ?? null)) {
            throw new InvalidArgumentException('Travis payload 缺少 commit.sha');
        }
        if (! is_string($raw['event_type'] ?? null)) {
            throw new InvalidArgumentException('Travis payload 缺少 event_type');
        }
        if (! is_string($raw['started_at'] ?? null)) {
            throw new InvalidArgumentException('Travis payload 缺少 started_at');
        }

        $branchName = is_array($branch) && is_string($branch['name'] ?? null)
            ? $branch['name']
            : null;

        $duration = $raw['duration'] ?? null;
        $durationSeconds = is_int($duration) ? $duration : null;

        $status = is_string($raw['state'] ?? null)
            ? BuildStatus::from($raw['state'])
            : throw new InvalidArgumentException('Travis payload 缺少 state');

        $id = $raw['id'] ?? null;
        $externalId = match (true) {
            is_int($id) => (string)$id,
            is_string($id) && $id !== '' => $id,
            default => throw new InvalidArgumentException('Travis payload 缺少 id'),
        };

        return new self(
            provider: CiProviderType::Travis,
            externalId: $externalId,
            repoFullName: $repository['slug'],
            commitSha: $commit['sha'],
            status: $status,
            eventType: $raw['event_type'],
            branch: $branchName,
            startedAt: CarbonImmutable::parse($raw['started_at'])->utc(),
            durationSeconds: $durationSeconds,
        );
    }
}
