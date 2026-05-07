<?php

declare(strict_types=1);

namespace App\Reporting\Renderer;

use App\Aggregation\Dto\FailureRateResult;
use Illuminate\Support\Collection;

final class FailureRateRenderer
{
    /**
     * 把失敗率聚合結果渲染成 markdown 表格（成員 × repo，附 Overall 欄）。
     *
     * @param Collection<int, FailureRateResult> $results
     */
    public function render(Collection $results): string
    {
        if ($results->isEmpty()) {
            return "## CI 失敗率\n\n（本月無資料）\n";
        }

        $repos = $results->map(fn (FailureRateResult $r) => (string)$r->repoFullName)
            ->unique()
            ->sort()
            ->values()
            ->all();
        $authors = $results->map(fn (FailureRateResult $r) => (string)$r->authorAccount)
            ->unique()
            ->sort()
            ->values()
            ->all();

        $byKey = [];
        foreach ($results as $r) {
            $byKey[(string)$r->authorAccount][(string)$r->repoFullName] = $r;
        }

        $header = array_merge(['author'], $repos, ['Overall']);
        $separator = array_map(fn () => '---', $header);

        $rows = [];
        foreach ($authors as $author) {
            $row = [$author];
            $totalBuilds = 0;
            $totalFailures = 0;
            foreach ($repos as $repo) {
                $cell = $byKey[$author][$repo] ?? null;
                $row[] = $cell === null ? '—' : $this->formatRate($cell->rate, $cell->failures, $cell->total);
                if ($cell !== null) {
                    $totalBuilds += $cell->total;
                    $totalFailures += $cell->failures;
                }
            }
            $overallRate = $totalBuilds > 0 ? $totalFailures / $totalBuilds : 0.0;
            $row[] = $totalBuilds > 0 ? $this->formatRate($overallRate, $totalFailures, $totalBuilds) : '—';
            $rows[] = $row;
        }

        $lines = ["## CI 失敗率", '', '| ' . implode(' | ', $header) . ' |', '| ' . implode(' | ', $separator) . ' |'];
        foreach ($rows as $row) {
            $lines[] = '| ' . implode(' | ', $row) . ' |';
        }
        $lines[] = '';

        return implode("\n", $lines);
    }

    private function formatRate(float $rate, int $failures, int $total): string
    {
        return sprintf('%.1f%% (%d/%d)', $rate * 100, $failures, $total);
    }
}
