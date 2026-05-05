<?php

declare(strict_types=1);

namespace App\Persistence\Enum;

enum Dataset: string
{
    case Builds = 'builds';
    case PullRequests = 'pull_requests';
}
