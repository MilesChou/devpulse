<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use App\Domain\Shared\CommitSha;
use App\Domain\Shared\RepoFullName;
use App\Domain\Vcs\Author;
use App\Domain\Vcs\PullRequestNumber;
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
