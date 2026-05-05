<?php

declare(strict_types=1);

namespace App\Persistence;

use App\Models\MonthFetch;
use App\Support\Time\MonthRange;
use Carbon\CarbonImmutable;

/**
 * 「已過月份不重撈」的決策層。
 *
 * 規則：
 * - 該月已撈完（status=complete） → 不重撈
 * - 該月還沒撈過或撈到一半（partial） → 撈
 * - 當月（即 month 還沒結束）→ 一律允許撈，因為資料還在累積
 */
final class MonthFetchCache
{
    public function __construct(private readonly ?CarbonImmutable $now = null)
    {
    }

    public function shouldFetch(int $repoId, string $dataset, string $month): bool
    {
        if ($this->isCurrentMonth($month)) {
            return true;
        }

        $record = MonthFetch::query()
            ->where('repo_id', $repoId)
            ->where('dataset', $dataset)
            ->where('month', $month)
            ->first();

        if ($record === null) {
            return true;
        }

        return $record->status !== MonthFetch::STATUS_COMPLETE;
    }

    public function markComplete(int $repoId, string $dataset, string $month): void
    {
        MonthFetch::query()->updateOrCreate(
            ['repo_id' => $repoId, 'dataset' => $dataset, 'month' => $month],
            ['status' => MonthFetch::STATUS_COMPLETE, 'fetched_at' => $this->now()],
        );
    }

    public function markPartial(int $repoId, string $dataset, string $month): void
    {
        MonthFetch::query()->updateOrCreate(
            ['repo_id' => $repoId, 'dataset' => $dataset, 'month' => $month],
            ['status' => MonthFetch::STATUS_PARTIAL, 'fetched_at' => $this->now()],
        );
    }

    private function isCurrentMonth(string $month): bool
    {
        [$start, $end] = MonthRange::parse($month);
        $now = $this->now();

        return $now->greaterThanOrEqualTo($start) && $now->lessThan($end);
    }

    private function now(): CarbonImmutable
    {
        return $this->now ?? CarbonImmutable::now('UTC');
    }
}
