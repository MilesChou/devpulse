<?php

declare(strict_types=1);

namespace App\Support\Time;

use Carbon\CarbonImmutable;
use InvalidArgumentException;

final class MonthRange
{
    /**
     * 把 YYYY-MM 月份字串解析成 [月初 UTC, 下個月初 UTC]，半開區間 [start, end)。
     *
     * @return array{0: CarbonImmutable, 1: CarbonImmutable}
     */
    public static function parse(string $month): array
    {
        $parsed = CarbonImmutable::createFromFormat('Y-m', $month, 'UTC');
        if (! $parsed instanceof CarbonImmutable) {
            throw new InvalidArgumentException("month 格式錯誤：必須是 YYYY-MM（收到 `{$month}`）");
        }
        $start = $parsed->startOfMonth();

        return [$start, $start->addMonth()];
    }
}
