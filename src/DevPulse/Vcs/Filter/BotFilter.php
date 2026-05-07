<?php

declare(strict_types=1);

namespace DevPulse\Vcs\Filter;

use DevPulse\Vcs\PullRequest;
use DevPulse\Vcs\ReviewSummary;

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
        return $this->isExcluded((string)$pr->author);
    }

    public function isBotReview(ReviewSummary $review): bool
    {
        return $this->isExcluded($review->reviewerAccount);
    }
}
