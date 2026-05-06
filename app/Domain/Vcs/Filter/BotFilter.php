<?php

declare(strict_types=1);

namespace App\Domain\Vcs\Filter;

use App\Domain\Vcs\PullRequest;
use App\Domain\Vcs\ReviewSummary;

final readonly class BotFilter
{
    /**
     * @param list<string> $excludedAccounts GitHub account 清單，命中即視為 bot
     */
    public function __construct(private iterable $excludedAccounts)
    {
    }

    public function isExcluded(string $account): bool
    {
        return in_array($account, $this->excludedAccounts, true);
    }

    public function isBotPullRequest(PullRequest $pr): bool
    {
        return $this->isExcluded($pr->author->toString());
    }

    public function isBotReview(ReviewSummary $review): bool
    {
        return $this->isExcluded($review->reviewerAccount);
    }
}
