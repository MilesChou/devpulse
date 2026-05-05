<?php

declare(strict_types=1);

namespace App\Persistence;

use App\Models\MonthFetch;
use App\Persistence\Enum\Dataset;
use App\Persistence\Enum\MonthFetchStatus;
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

    public function shouldFetch(int $repoId, Dataset $dataset, string $month): bool
    {
        if ($this->isCurrentMonth($month)) {
            return true;
        }

        $record = MonthFetch::query()
            ->where('repo_id', $repoId)
            ->where('dataset', $dataset->value)
            ->where('month', $month)
            ->first();

        if ($record === null) {
            return true;
        }

        return $record->status !== MonthFetchStatus::Complete;
    }

    public function markComplete(int $repoId, Dataset $dataset, string $month): void
    {
        $this->markStatus($repoId, $dataset, $month, MonthFetchStatus::Complete);
    }

    public function markPartial(int $repoId, Dataset $dataset, string $month): void
    {
        $this->markStatus($repoId, $dataset, $month, MonthFetchStatus::Partial);
    }

    private function markStatus(int $repoId, Dataset $dataset, string $month, MonthFetchStatus $status): void
    {
        MonthFetch::query()->updateOrCreate(
            ['repo_id' => $repoId, 'dataset' => $dataset->value, 'month' => $month],
            ['status' => $status->value, 'fetched_at' => $this->now()],
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
