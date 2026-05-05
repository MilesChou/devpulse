<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

final readonly class PrBuildCountResult
{
    public function __construct(
        public string $repoFullName,
        public int $prNumber,
        public int $buildCount,
    ) {
    }
}
