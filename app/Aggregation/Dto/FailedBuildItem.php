<?php

declare(strict_types=1);

namespace App\Aggregation\Dto;

use App\Domain\Shared\RepoFullName;
use Carbon\CarbonImmutable;

final readonly class FailedBuildItem
{
    public function __construct(
        public RepoFullName $repoFullName,
        public string $externalId,
        public string $commitSha,
        public ?string $authorAccount,
        public ?int $prNumber,
        public string $status,
        public CarbonImmutable $startedAt,
    ) {
    }
}
