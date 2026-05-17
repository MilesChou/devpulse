<?php

declare(strict_types=1);

namespace App\Fetching;

final readonly class RepoFetchOutcome
{
    public function __construct(
        public string $repoFullName,
        public int $buildsWritten,
        public int $pullRequestsWritten,
        public ?string $error = null,
    ) {
    }
}
