<?php

declare(strict_types=1);

namespace App\Services\Web;

use App\Models\Build;
use Carbon\CarbonImmutable;

final class PrErrorRateQuery
{
    /**
     * PR 錯誤率：失敗 build 數 / (總 build 數 - canceled)。
     *
     * 對應 Grafana panel 1「失敗率」。
     * 排除 is_post_merge 與 is_deploy_event。
     *
     * @return array{
     *     rate: float|null,
     *     fails: int,
     *     denom: int,
     * }
     */
    public function summary(CarbonImmutable $from, CarbonImmutable $to): array
    {
        // CASE 寫法跨 SQLite/Postgres；`is_failure::int` 只在 Postgres 通。
        $result = Build::query()
            ->whereBetween('started_at', [$from->startOfDay(), $to->endOfDay()])
            ->where('is_post_merge', false)
            ->where('is_deploy_event', false)
            ->where('status', '<>', 'CANCELED')
            ->selectRaw('COUNT(*) AS denom, SUM(CASE WHEN is_failure THEN 1 ELSE 0 END) AS fails')
            ->first();

        $rawDenom = $result?->getAttribute('denom');
        $rawFails = $result?->getAttribute('fails');
        $denom = is_numeric($rawDenom) ? (int)$rawDenom : 0;
        $fails = is_numeric($rawFails) ? (int)$rawFails : 0;

        return [
            'rate' => $denom > 0 ? $fails / $denom : null,
            'fails' => $fails,
            'denom' => $denom,
        ];
    }
}
