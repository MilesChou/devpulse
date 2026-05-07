<?php

declare(strict_types=1);

namespace DevPulse\Ci;

enum BuildTrigger
{
    case PULL_REQUEST;
    case POST_MERGE;
    case SCHEDULED;
    case MANUAL;
}
