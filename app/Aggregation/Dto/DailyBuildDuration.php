<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

final readonly class DailyBuildDuration
{
    public function __construct(
        public string $repoFullName,
        public string $date,
        public int $count,
        public float $medianSeconds,
        public int $maxSeconds,
    ) {
    }
}
