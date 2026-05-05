<?php

declare(strict_types=1);

namespace App\Domain\Ci;

enum BuildTrigger
{
    case PULL_REQUEST;
    case POST_MERGE;
    case SCHEDULED;
    case MANUAL;
}
