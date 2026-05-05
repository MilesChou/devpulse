<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\FailureRateResult;
use App\Aggregation\Filter\BuildEventFilter;
use App\Domain\Shared\MonthRange;
use App\Domain\Shared\RepoFullName;
use App\Models\Build;
use App\Models\Group;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\DB;

final class BuildFailureRateQuery
{
    public function __construct(private readonly BuildEventFilter $filter)
    {
    }

    /**
     * @param string[]|null $repoFullNames 限縮 repo（null = group 所有 repo）
     * @param string[]|null $authorAccounts 限縮成員（null = group 所有成員）
     * @return Collection<int, FailureRateResult>
     */
    public function run(
        Group $group,
        MonthRange $month,
        ?array $repoFullNames = null,
        ?array $authorAccounts = null,
    ): Collection {
        $query = Build::query()
            ->join('repos', 'repos.id', '=', 'builds.repo_id')
            ->whereIn('builds.repo_id', $group->repoIds())
            ->where('builds.started_at', '>=', $month->start)
            ->where('builds.started_at', '<', $month->end)
            ->whereNotNull('builds.author_account');

        $this->filter->apply($query);

        if ($repoFullNames !== null) {
            $query->whereIn('repos.full_name', $repoFullNames);
        }

        if ($authorAccounts !== null) {
            $query->whereIn('builds.author_account', $authorAccounts);
        }

        $rows = $query
            ->select([
                'repos.full_name as repo_full_name',
                'builds.author_account',
                DB::raw('COUNT(*) as total'),
                DB::raw('SUM(CASE WHEN builds.is_failure THEN 1 ELSE 0 END) as failures'),
            ])
            ->groupBy('repos.full_name', 'builds.author_account')
            ->get();

        return $rows->map(fn ($row) => FailureRateResult::from(
            repoFullName: new RepoFullName($row->repo_full_name),
            authorAccount: $row->author_account,
            total: (int)$row->total,
            failures: (int)$row->failures,
        ));
    }
}
