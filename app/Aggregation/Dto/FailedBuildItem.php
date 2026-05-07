<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use DevPulse\Shared\CommitSha;
use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\Author;
use DevPulse\Vcs\PullRequestNumber;
use Carbon\CarbonImmutable;

final readonly class FailedBuildItem
{
    public function __construct(
        public RepoFullName $repoFullName,
        public string $externalId,
        public CommitSha $commitSha,
        public ?Author $authorAccount,
        public ?PullRequestNumber $prNumber,
        public string $status,
        public CarbonImmutable $startedAt,
    ) {
    }
}
