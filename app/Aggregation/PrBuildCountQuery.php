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
            ->with('repo')
            ->whereIn('repo_id', $group->repoIds())
            ->where('started_at', '>=', $month->start)
            ->where('started_at', '<', $month->end)
            ->where('is_pull_request', true)
            ->whereNotNull('pr_number')
            ->select([
                'repo_id',
                'pr_number',
                DB::raw('COUNT(*) as build_count'),
            ])
            ->groupBy('repo_id', 'pr_number')
            ->orderBy('repo_id')
            ->orderBy('pr_number')
            ->get();

        return $rows->map(fn (Build $row) => new PrBuildCountResult(
            repoFullName: new RepoFullName($row->repo->full_name),
            prNumber: (int)$row->pr_number,
            buildCount: (int)$row->build_count, // @phpstan-ignore property.notFound, cast.int
        ));
    }
}
