<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\FailureRateResult;
use App\Aggregation\Filter\BuildEventFilter;
use DevPulse\Shared\MonthRange;
use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\Author;
use App\Models\Build;
use App\Models\Group;
use Illuminate\Support\Collection;
use Illuminate\Support\Facades\DB;

class BuildFailureRateQuery
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
            ->join('dp_repos', 'dp_repos.id', '=', 'dp_builds.repo_id')
            ->whereIn('dp_builds.repo_id', $group->repoIds())
            ->where('dp_builds.started_at', '>=', $month->start)
            ->where('dp_builds.started_at', '<', $month->end)
            ->whereNotNull('dp_builds.author_account');

        $this->filter->apply($query);

        if ($repoFullNames !== null) {
            $query->whereIn('dp_repos.name', $repoFullNames);
        }

        if ($authorAccounts !== null) {
            $query->whereIn('dp_builds.author_account', $authorAccounts);
        }

        /** @var \Illuminate\Support\Collection<int, object{repo_full_name: string, author_account: string, total: int, failures: int}> $rows */
        $rows = $query
            ->select([
                'dp_repos.name as repo_full_name',
                'dp_builds.author_account',
                DB::raw('COUNT(*) as total'),
                DB::raw('SUM(CASE WHEN dp_builds.is_failure THEN 1 ELSE 0 END) as failures'),
            ])
            ->groupBy('dp_repos.name', 'dp_builds.author_account')
            ->get();

        return $rows->map(fn ($row) => FailureRateResult::from(
            repoFullName: new RepoFullName($row->repo_full_name),
            authorAccount: new Author($row->author_account),
            total: (int)$row->total,
            failures: (int)$row->failures,
        ));
    }
}
