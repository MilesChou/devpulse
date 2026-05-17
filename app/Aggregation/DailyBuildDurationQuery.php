<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\DailyBuildDuration;
use DevPulse\Ci\BuildStatus;
use DevPulse\Shared\MonthRange;
use DevPulse\Shared\RepoFullName;
use App\Models\Build;
use App\Models\Group;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

class DailyBuildDurationQuery
{
    /**
     * 計算指定月份、指定 group 內每個 repo 每日「通過 build」的時間統計。
     *
     * @return Collection<int, DailyBuildDuration>
     */
    public function run(Group $group, MonthRange $month): Collection
    {
        /** @var \Illuminate\Support\Collection<int, object{repo_full_name: string, duration_seconds: int, started_at: string}> $builds */
        $builds = Build::query()
            ->whereIn('repo_id', $group->repoIds())
            ->where('started_at', '>=', $month->start)
            ->where('started_at', '<', $month->end)
            ->where('status', BuildStatus::PASSED->name)
            ->whereNotNull('duration_seconds')
            ->join('dp_repos', 'dp_repos.id', '=', 'dp_builds.repo_id')
            ->select(['dp_repos.name as repo_full_name', 'dp_builds.duration_seconds', 'dp_builds.started_at'])
            ->orderBy('dp_repos.name')
            ->orderBy('dp_builds.started_at')
            ->get();

        $grouped = $builds->groupBy(
            fn ($row) => $row->repo_full_name . '|' . CarbonImmutable::parse($row->started_at)->format('Y-m-d'),
        );

        return $grouped->map(function (Collection $group): DailyBuildDuration {
            $first = $group->first();
            assert($first !== null);
            /** @phpstan-ignore cast.int */
            $durations = $group->pluck('duration_seconds')->map(fn ($v) => (int)$v)->sort()->values()->all();
            $date = CarbonImmutable::parse($first->started_at)->format('Y-m-d');

            return new DailyBuildDuration(
                repoFullName: new RepoFullName($first->repo_full_name),
                date: $date,
                count: count($durations),
                medianSeconds: $this->calcMedian($durations),
                maxSeconds: $durations !== [] ? max($durations) : 0,
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

        $mid = (int)($count / 2);

        if ($count % 2 === 0) {
            return ($sortedValues[$mid - 1] + $sortedValues[$mid]) / 2.0;
        }

        return (float)$sortedValues[$mid];
    }
}
