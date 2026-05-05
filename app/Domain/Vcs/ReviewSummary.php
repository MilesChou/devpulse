<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use Carbon\CarbonImmutable;
use InvalidArgumentException;

final readonly class ReviewSummary
{
    public function __construct(
        public string $repoFullName,
        public int $pullRequestNumber,
        public string $reviewerAccount,
        public ReviewState $state,
        public CarbonImmutable $submittedAt,
    ) {
        if (! str_contains($repoFullName, '/')) {
            throw new InvalidArgumentException('repoFullName 必須是 owner/name 格式');
        }
        if ($pullRequestNumber < 1) {
            throw new InvalidArgumentException('pullRequestNumber 必須 >= 1');
        }
        if ($reviewerAccount === '') {
            throw new InvalidArgumentException('reviewerAccount 不能是空字串');
        }
    }
}
