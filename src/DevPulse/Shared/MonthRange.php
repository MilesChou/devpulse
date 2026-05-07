<?php

declare(strict_types=1);

namespace DevPulse\Shared;

use Carbon\CarbonImmutable;
use InvalidArgumentException;

final class MonthRange
{
    public readonly CarbonImmutable $start;
    public readonly CarbonImmutable $end;

    private function __construct(CarbonImmutable $start, CarbonImmutable $end)
    {
        $this->start = $start;
        $this->end = $end;
    }

    public static function fromString(string $month): self
    {
        $parsed = CarbonImmutable::createFromFormat('Y-m', $month, 'UTC');
        if (! $parsed instanceof CarbonImmutable) {
            throw new InvalidArgumentException("month must be YYYY-MM format (got `{$month}`)");
        }
        $start = $parsed->startOfMonth();

        return new self($start, $start->addMonth());
    }

    /** 半開區間 [start, end) */
    public function contains(CarbonImmutable $dt): bool
    {
        return $dt->greaterThanOrEqualTo($this->start) && $dt->lessThan($this->end);
    }
}
