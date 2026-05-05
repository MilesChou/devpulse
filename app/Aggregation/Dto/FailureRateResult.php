<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use App\Domain\Shared\RepoFullName;

final readonly class FailureRateResult
{
    public function __construct(
        public RepoFullName $repoFullName,
        public string $authorAccount,
        public int $total,
        public int $failures,
        public float $rate,
    ) {
    }

    public static function from(RepoFullName $repoFullName, string $authorAccount, int $total, int $failures): self
    {
        return new self(
            repoFullName: $repoFullName,
            authorAccount: $authorAccount,
            total: $total,
            failures: $failures,
            rate: $total > 0 ? $failures / $total : 0.0,
        );
    }
}
