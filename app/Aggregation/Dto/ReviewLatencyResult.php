<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\Author;
use DevPulse\Vcs\PullRequestNumber;

final readonly class ReviewLatencyResult
{
    public function __construct(
        public RepoFullName $repoFullName,
        public PullRequestNumber $prNumber,
        public Author $authorAccount,
        public string $sizeBucket,
        public float $latencyHours,
        public bool $isLowerBound,
    ) {
    }
}
