<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use App\Domain\Shared\RepoFullName;
use App\Domain\Vcs\PullRequestNumber;

final readonly class PrBuildCountResult
{
    public function __construct(
        public RepoFullName $repoFullName,
        public PullRequestNumber $prNumber,
        public int $buildCount,
    ) {
    }
}
