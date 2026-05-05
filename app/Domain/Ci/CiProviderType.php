<?php

declare(strict_types=1);

namespace App\Domain\Ci;

enum CiProviderType: string
{
    case Travis = 'travis';
    case GitHubActions = 'github_actions';
}
