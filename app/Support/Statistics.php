<?php

declare(strict_types=1);

namespace App\Support;

final class Statistics
{
    /**
     * 計算連續百分位（線性插值法），對齊 PostgreSQL `PERCENTILE_CONT($p)`。
     *
     * 與 ReviewLatencyRenderer 的算法一致；Grafana dashboard 也用此版本。
     *
     * @param list<int|float> $values 任意順序的值；本函式自行排序
     * @param float $p 百分位（0.0–1.0）
     */
    public static function percentile(array $values, float $p): ?float
    {
        if ($values === []) {
            return null;
        }

        sort($values);
        $count = count($values);
        if ($count === 1) {
            return (float)$values[0];
        }

        $index = ($count - 1) * $p;
        $lower = (int)floor($index);
        $upper = (int)ceil($index);

        if ($lower === $upper) {
            return (float)$values[$lower];
        }

        $fraction = $index - $lower;
        return (float)$values[$lower]
            + ((float)$values[$upper] - (float)$values[$lower]) * $fraction;
    }
}
