<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

final readonly class ReviewLatencyResult
{
    public function __construct(
        public string $repoFullName,
        public int $prNumber,
        public string $authorAccount,
        public string $sizeBucket,
        public float $latencyHours,
        public bool $isLowerBound,
    ) {
    }
}
