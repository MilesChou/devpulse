<?php

declare(strict_types=1);

namespace DevPulse\Shared;

use DateTimeImmutable;
use DateTimeZone;
use InvalidArgumentException;

final class MonthRange
{
    public function __construct(
        public readonly DateTimeImmutable $start,
        public readonly DateTimeImmutable $end,
    ) {
    }

    public static function fromString(string $month): self
    {
        if (! preg_match('/^\d{4}-(0[1-9]|1[0-2])$/', $month)) {
            throw new InvalidArgumentException("month must be YYYY-MM format (got `{$month}`)");
        }

        $start = DateTimeImmutable::createFromFormat('!Y-m', $month, new DateTimeZone('UTC'));
        if ($start === false) {
            throw new InvalidArgumentException("month must be YYYY-MM format (got `{$month}`)");
        }

        return new self($start, $start->modify('+1 month'));
    }

    /** 半開區間 [start, end) */
    public function contains(DateTimeImmutable $dt): bool
    {
        return $dt >= $this->start && $dt < $this->end;
    }
}
