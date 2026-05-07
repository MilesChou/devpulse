<?php

declare(strict_types=1);

namespace DevPulse\Vcs;

use DevPulse\Shared\RepoId;
use Carbon\CarbonImmutable;
use InvalidArgumentException;

/**
 * PR 實體，以 (repoId, number) 為識別，狀態會隨 merge / close / ready-for-review 異動。
 */
final class PullRequest
{
    private PullRequestStatus $status;
    private ChangeStats $changes;
    private ?CarbonImmutable $readyAt;
    private ?CarbonImmutable $closedAt;

    public function __construct(
        public readonly PullRequestId $id,
        public readonly Platform $platform,
        public readonly RepoId $repoId,
        public readonly PullRequestNumber $number,
        public readonly Author $author,
        PullRequestStatus $status,
        ChangeStats $changes,
        public readonly CarbonImmutable $createdAt,
        ?CarbonImmutable $readyAt,
        ?CarbonImmutable $closedAt,
    ) {
        if (! $status->isOpen() && $closedAt === null) {
            throw new InvalidArgumentException('closedAt must be set when status is closed or merged');
        }

        $this->status = $status;
        $this->changes = $changes;
        $this->readyAt = $readyAt;
        $this->closedAt = $closedAt;
    }

    public function status(): PullRequestStatus
    {
        return $this->status;
    }

    public function changes(): ChangeStats
    {
        return $this->changes;
    }

    public function readyAt(): ?CarbonImmutable
    {
        return $this->readyAt;
    }

    public function closedAt(): ?CarbonImmutable
    {
        return $this->closedAt;
    }

    public function merge(CarbonImmutable $closedAt): void
    {
        $this->status = PullRequestStatus::Merged;
        $this->closedAt = $closedAt;
    }

    public function close(CarbonImmutable $closedAt): void
    {
        $this->status = PullRequestStatus::Closed;
        $this->closedAt = $closedAt;
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
