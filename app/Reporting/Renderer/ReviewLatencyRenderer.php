<?php

declare(strict_types=1);

namespace App\Reporting\Renderer;

use App\Aggregation\Dto\ReviewLatencyResult;
use Illuminate\Support\Collection;

final class ReviewLatencyRenderer
{
    /** size bucket 顯示順序 */
    private const BUCKET_ORDER = ['XS', 'S', 'M', 'L', 'XL'];

    /**
     * 把 PR review latency 渲染成 markdown 表格（依 size bucket 切片）。
     *
     * @param Collection<int, ReviewLatencyResult> $results
     */
    public function render(Collection $results): string
    {
        if ($results->isEmpty()) {
            return "## PR Review Latency\n\n（本月無資料）\n";
        }

        $byBucket = $results->groupBy(fn (ReviewLatencyResult $r) => $r->sizeBucket);

        $lines = [
            '## PR Review Latency',
            '',
            '| Size | PR 數 | 中位數 (h) | P75 (h) | 最大 (h) | 含 lower bound |',
            '| --- | --- | --- | --- | --- | --- |',
        ];

        foreach (self::BUCKET_ORDER as $bucket) {
            if (!$byBucket->has($bucket)) {
                continue;
            }
            $group = $byBucket->get($bucket);
            $hours = $group->map(fn (ReviewLatencyResult $r) => $r->latencyHours)
                ->sort()
                ->values()
                ->all();
            $count = count($hours);
            $lowerBoundCount = $group->filter(fn (ReviewLatencyResult $r) => $r->isLowerBound)->count();

            $median = $this->percentile($hours, 0.5);
            $p75 = $this->percentile($hours, 0.75);
            $max = $hours[$count - 1];

            $lines[] = sprintf(
                '| %s | %d | %.1f | %.1f | %.1f | %d |',
                $bucket,
                $count,
                $median,
                $p75,
                $max,
                $lowerBoundCount,
            );
        }

        $lines[] = '';

        return implode("\n", $lines);
    }

    /**
     * @param float[] $sortedValues 已排序的值（升冪）
     */
    private function percentile(array $sortedValues, float $p): float
    {
        $count = count($sortedValues);
        if ($count === 0) {
            return 0.0;
        }
        if ($count === 1) {
            return $sortedValues[0];
        }

        $index = ($count - 1) * $p;
        $lower = (int)floor($index);
        $upper = (int)ceil($index);

        if ($lower === $upper) {
            return $sortedValues[$lower];
        }

        $fraction = $index - $lower;

        return $sortedValues[$lower] + ($sortedValues[$upper] - $sortedValues[$lower]) * $fraction;
    }
}
