<?php

declare(strict_types=1);

namespace App\Domain\Ci;

enum BuildStatus: string
{
    case Passed = 'passed';
    case Failed = 'failed';
    case Errored = 'errored';
    case Canceled = 'canceled';
    case Created = 'created';
    case Started = 'started';

    public function isFailure(): bool
    {
        return $this === self::Failed || $this === self::Errored;
    }

    public function isComplete(): bool
    {
        return $this !== self::Created && $this !== self::Started;
    }
}
