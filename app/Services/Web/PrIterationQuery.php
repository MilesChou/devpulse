<?php

declare(strict_types=1);

namespace App\Services\Web;

use App\Models\Build;
use Carbon\CarbonImmutable;

final class PrIterationQuery
{
    /**
     * PR 重推次數：每個 PR 平均的 build 次數。
     *
     * 對應 Grafana panel 3「PR 迭代次數」：total_builds / total_PRs，
     * 限定 is_pull_request = true 且 pr_number IS NOT NULL。
     *
     * @return array{
     *     ratio: float|null,
     *     builds: int,
     *     prs: int,
     * }
     */
    public function summary(CarbonImmutable $from, CarbonImmutable $to): array
    {
        // 用子查詢一次拉回 builds / prs；避免把每個 (repo_id, pr_number)
        // 群組都搬回 PHP 再加總。COUNT(DISTINCT repo_id || ',' || pr_number)
        // 在 SQLite/Postgres 都通。
        $result = Build::query()
            ->whereBetween('started_at', [$from->startOfDay(), $to->endOfDay()])
            ->where('is_pull_request', true)
            ->whereNotNull('pr_number')
            ->selectRaw(
                'COUNT(*) AS builds, '
                . "COUNT(DISTINCT repo_id || ',' || pr_number) AS prs",
            )
            ->first();

        $rawBuilds = $result?->getAttribute('builds');
        $rawPrs = $result?->getAttribute('prs');
        $builds = is_numeric($rawBuilds) ? (int)$rawBuilds : 0;
        $prs = is_numeric($rawPrs) ? (int)$rawPrs : 0;

        return [
            'ratio' => $prs > 0 ? $builds / $prs : null,
            'builds' => $builds,
            'prs' => $prs,
        ];
    }
}
