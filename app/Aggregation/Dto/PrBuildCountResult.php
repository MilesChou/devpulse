<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use App\Domain\Shared\RepoFullName;

final readonly class PrBuildCountResult
{
    public function __construct(
        public RepoFullName $repoFullName,
        public int $prNumber,
        public int $buildCount,
    ) {
    }
}
