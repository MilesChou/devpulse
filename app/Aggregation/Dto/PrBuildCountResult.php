<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\PullRequestNumber;

final readonly class PrBuildCountResult
{
    public function __construct(
        public RepoFullName $repoFullName,
        public PullRequestNumber $prNumber,
        public int $buildCount,
    ) {
    }
}
