<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\PrBuildCountResult;
use App\Domain\Shared\MonthRange;
use App\Domain\Shared\RepoFullName;
use App\Models\Build;
use App\Models\Group;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\DB;

final class PrBuildCountQuery
{
    /**
     * @return Collection<int, PrBuildCountResult>
     */
    public function run(Group $group, MonthRange $month): Collection
    {
        $rows = Build::query()
            ->whereIn('repo_id', $group->repoIds())
            ->where('started_at', '>=', $month->start)
            ->where('started_at', '<', $month->end)
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
            repoFullName: new RepoFullName($row->repo_full_name),
            prNumber: (int)$row->pr_number,
            buildCount: (int)$row->build_count,
        ));
    }

    /**
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
