<?php

declare(strict_types=1);

namespace DevPulse\Ci;

enum BuildStatus
{
    case PASSED;
    case FAILED;
    case CANCELED;
    case IN_PROGRESS;

    public function isFailure(): bool
    {
        return $this === self::FAILED;
    }

    public function isComplete(): bool
    {
        return $this !== self::IN_PROGRESS;
    }
}
