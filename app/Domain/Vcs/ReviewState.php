<?php

declare(strict_types=1);

namespace App\Domain\Vcs;

enum ReviewState: string
{
    case Approved = 'approved';
    case ChangesRequested = 'changes_requested';
    case Commented = 'commented';
    case Dismissed = 'dismissed';
    case Pending = 'pending';
}
