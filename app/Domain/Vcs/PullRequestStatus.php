<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use InvalidArgumentException;

/**
 * PR 的生命週期狀態；值與 GitHub REST API 的 state 欄位對應（Merged 為本系統擴充）。
 */
enum PullRequestStatus: string
{
    case Open = 'open';
    case Closed = 'closed';
    case Merged = 'merged';

    public function isMerged(): bool
    {
        return $this === self::Merged;
    }

    public function isOpen(): bool
    {
        return $this === self::Open;
    }

    /**
     * 從 GitHub REST API 的 state 和 merged_at 推斷 PR 狀態。
     * GitHub 將已合併的 PR 標記為 "closed"，以 merged_at 區分。
     */
    public static function fromGitHubState(string $state, ?string $mergedAt): self
    {
        if ($mergedAt !== null) {
            return self::Merged;
        }

        return match ($state) {
            'open' => self::Open,
            'closed' => self::Closed,
            default => throw new InvalidArgumentException("Unknown GitHub PR state: $state"),
        };
    }
}
