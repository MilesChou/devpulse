<?php

declare(strict_types=1);

namespace App\Reporting\Renderer;

use App\Aggregation\Dto\DailyBuildDuration;
use Illuminate\Support\Collection;

final class DailyBuildDurationRenderer
{
    private const BAR_WIDTH = 30;

    /**
     * 把每日 build 時間渲染成 ASCII bar chart + 表格。
     *
     * @param Collection<int, DailyBuildDuration> $results
     */
    public function render(Collection $results): string
    {
        if ($results->isEmpty()) {
            return "## 每日 Build 時間趨勢\n\n（本月無資料）\n";
        }

        $byRepo = $results->groupBy(fn (DailyBuildDuration $r) => (string)$r->repoFullName);
        $lines = ['## 每日 Build 時間趨勢', ''];

        foreach ($byRepo as $repoFullName => $rows) {
            $sorted = $rows->sortBy('date')->values();
            $maxMedian = max($sorted->map(fn (DailyBuildDuration $r) => $r->medianSeconds)->all());

            $lines[] = "### {$repoFullName}";
            $lines[] = '';
            $lines[] = '| 日期 | 筆數 | 中位數 | 最大 | 趨勢 |';
            $lines[] = '| --- | --- | --- | --- | --- |';

            foreach ($sorted as $row) {
                $bar = $this->renderBar($row->medianSeconds, $maxMedian);
                $lines[] = sprintf(
                    '| %s | %d | %s | %s | `%s` |',
                    $row->date,
                    $row->count,
                    $this->formatSeconds((int)round($row->medianSeconds)),
                    $this->formatSeconds($row->maxSeconds),
                    $bar,
                );
            }
            $lines[] = '';
        }

        return implode("\n", $lines);
    }

    private function renderBar(float $value, float $max): string
    {
        if ($max <= 0) {
            return str_repeat('·', self::BAR_WIDTH);
        }
        $width = (int)round($value / $max * self::BAR_WIDTH);
        $width = max(1, $width);

        return str_repeat('█', $width) . str_repeat('·', self::BAR_WIDTH - $width);
    }

    private function formatSeconds(int $seconds): string
    {
        if ($seconds < 60) {
            return "{$seconds}s";
        }
        $minutes = intdiv($seconds, 60);
        $remainder = $seconds % 60;

        return $remainder === 0 ? "{$minutes}m" : "{$minutes}m{$remainder}s";
    }
}
