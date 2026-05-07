<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use App\Domain\Shared\RepoFullName;
use App\Domain\Vcs\Author;
use App\Domain\Vcs\PullRequestNumber;

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
