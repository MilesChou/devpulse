<?php

declare(strict_types=1);

namespace App\Domain\Ci;

enum CiProvider: string
{
    case Travis = 'travis';
    case GitHubActions = 'github_actions';
}
