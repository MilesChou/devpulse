<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

/** PR review 的審查結果狀態；值為小寫，對應 GitHub GraphQL 的 PullRequestReviewState（大寫）。 */
enum ReviewState: string
{
    case Approved = 'approved';
    case ChangesRequested = 'changes_requested';
    case Commented = 'commented';
    case Dismissed = 'dismissed';
    case Pending = 'pending';

    /**
     * 從 GitHub GraphQL 的大寫 enum 字串建立。
     */
    public static function fromGitHubGraphQL(string $value): self
    {
        return self::from(strtolower($value));
    }
}
