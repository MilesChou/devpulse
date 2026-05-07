<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

use App\Domain\Shared\RepoId;
use Carbon\CarbonImmutable;
use InvalidArgumentException;

/**
 * 一筆 PR 的快照摘要，記錄識別資訊、作者、狀態與關鍵時間戳。
 */
final readonly class PullRequest
{
    public function __construct(
        public RepoId $repoId,
        public PullRequestNumber $number,
        public Author $author,
        public PullRequestStatus $status,
        public ChangeStats $changes,
        public CarbonImmutable $createdAt,
        public ?CarbonImmutable $readyAt,
        public ?CarbonImmutable $mergedAt,
        public ?CarbonImmutable $closedAt,
    ) {
        if ($status->isMerged() && $mergedAt === null) {
            throw new InvalidArgumentException('mergedAt must be set when status is merged');
        }
        if (! $status->isOpen() && $closedAt === null) {
            throw new InvalidArgumentException('closedAt must be set when status is closed or merged');
        }
    }

    public function isDraft(): bool
    {
        return $this->readyAt === null;
    }

    public function totalChangedLines(): int
    {
        return $this->changes->total();
    }
}
