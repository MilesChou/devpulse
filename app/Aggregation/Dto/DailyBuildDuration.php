<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use DevPulse\Shared\RepoFullName;

final readonly class DailyBuildDuration
{
    public function __construct(
        public RepoFullName $repoFullName,
        public string $date,
        public int $count,
        public float $medianSeconds,
        public int $maxSeconds,
    ) {
    }
}
