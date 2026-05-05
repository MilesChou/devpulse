<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\PrBuildCountResult;
use App\Models\Build;
use App\Models\Group;
use App\Support\Time\MonthRange;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\DB;

final class PrBuildCountAggregator
{
    /**
     * @param string $month YYYY-MM
     * @return Collection<int, PrBuildCountResult>
     */
    public function aggregate(Group $group, string $month): Collection
    {
        [$start, $end] = MonthRange::parse($month);

        $groupRepoIds = $group->repos()->pluck('repos.id');

        $rows = Build::query()
            ->whereIn('repo_id', $groupRepoIds)
            ->where('started_at', '>=', $start)
            ->where('started_at', '<', $end)
            ->where('is_pull_request', true)
            ->whereNotNull('pr_number')
            ->join('repos', 'repos.id', '=', 'builds.repo_id')
            ->select([
                'repos.full_name as repo_full_name',
                'builds.pr_number',
                DB::raw('COUNT(*) as build_count'),
            ])
            ->groupBy('repos.full_name', 'builds.pr_number')
            ->orderBy('repos.full_name')
            ->orderBy('builds.pr_number')
            ->get();

        return $rows->map(fn ($row) => new PrBuildCountResult(
            repoFullName: $row->repo_full_name,
            prNumber: (int) $row->pr_number,
            buildCount: (int) $row->build_count,
        ));
    }

    /**
     * 計算指定月份的平均 PR build 次數（跨所有 PR）。
     *
     * @param Collection<int, PrBuildCountResult> $results
     */
    public static function averageBuildCount(Collection $results): float
    {
        $count = $results->count();
        if ($count === 0) {
            return 0.0;
        }

        return $results->sum(fn (PrBuildCountResult $r) => $r->buildCount) / $count;
    }
}
