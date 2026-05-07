<?php

declare(strict_types=1);

namespace App\Domain\Ci;

use App\Domain\Shared\CommitSha;
use App\Domain\Shared\RepoFullName;
use App\Domain\Vcs\PullRequestNumber;
use Carbon\CarbonImmutable;
use InvalidArgumentException;

final readonly class Build
{
    public function __construct(
        public string $externalId,
        public RepoFullName $repoFullName,
        public CommitSha $commitSha,
        public ?string $authorAccount,
        public ?PullRequestNumber $prNumber,
        public BuildStatus $status,
        public BuildTrigger $trigger,
        public ?string $branch,
        public CarbonImmutable $startedAt,
        public ?int $durationSeconds,
    ) {
        if ($externalId === '') {
            throw new InvalidArgumentException('externalId must not be empty');
        }
        if ($durationSeconds !== null && $durationSeconds < 0) {
            throw new InvalidArgumentException('durationSeconds must not be negative');
        }
    }

    public function isPostMerge(): bool
    {
        return $this->trigger === BuildTrigger::POST_MERGE;
    }

    public function isPullRequest(): bool
    {
        return $this->trigger === BuildTrigger::PULL_REQUEST;
    }

    public function isDeployEvent(): bool
    {
        return $this->trigger === BuildTrigger::SCHEDULED
            || $this->trigger === BuildTrigger::MANUAL;
    }

    public function isFailure(): bool
    {
        return $this->status->isFailure();
    }
}
