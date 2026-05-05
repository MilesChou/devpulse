<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\DailyBuildDuration;
use App\Domain\Ci\BuildStatus;
use App\Models\Build;
use App\Models\Group;
use App\Support\Time\MonthRange;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

final class DailyBuildDurationAggregator
{
    /**
     * 計算指定月份、指定 group 內每個 repo 每日「通過 build」的時間統計。
     *
     * @param string $month YYYY-MM
     * @return Collection<int, DailyBuildDuration>
     */
    public function aggregate(Group $group, string $month): Collection
    {
        [$start, $end] = MonthRange::parse($month);

        $groupRepoIds = $group->repos()->pluck('repos.id');

        $builds = Build::query()
            ->whereIn('repo_id', $groupRepoIds)
            ->where('started_at', '>=', $start)
            ->where('started_at', '<', $end)
            ->where('status', BuildStatus::Passed->value)
            ->whereNotNull('duration_seconds')
            ->join('repos', 'repos.id', '=', 'builds.repo_id')
            ->select(['repos.full_name as repo_full_name', 'builds.duration_seconds', 'builds.started_at'])
            ->orderBy('repos.full_name')
            ->orderBy('builds.started_at')
            ->get();

        $grouped = $builds->groupBy(fn ($row) => $row->repo_full_name . '|' . CarbonImmutable::parse($row->started_at)->format('Y-m-d'));

        return $grouped->map(function (Collection $group): DailyBuildDuration {
            $first = $group->first();
            $durations = $group->pluck('duration_seconds')->map(fn ($v) => (int) $v)->sort()->values()->all();
            $date = CarbonImmutable::parse($first->started_at)->format('Y-m-d');

            return new DailyBuildDuration(
                repoFullName: $first->repo_full_name,
                date: $date,
                count: count($durations),
                medianSeconds: $this->calcMedian($durations),
                maxSeconds: max($durations),
            );
        })->values();
    }

    /** @param int[] $sortedValues */
    private function calcMedian(array $sortedValues): float
    {
        $count = count($sortedValues);
        if ($count === 0) {
            return 0.0;
        }

        $mid = (int) ($count / 2);

        if ($count % 2 === 0) {
            return ($sortedValues[$mid - 1] + $sortedValues[$mid]) / 2.0;
        }

        return (float) $sortedValues[$mid];
    }
}
