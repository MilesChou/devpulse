<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

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
}
