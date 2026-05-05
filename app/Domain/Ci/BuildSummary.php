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
}
