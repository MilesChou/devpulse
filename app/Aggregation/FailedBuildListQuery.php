<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\FailedBuildItem;
use App\Aggregation\Filter\BuildEventFilter;
use DevPulse\Shared\CommitSha;
use DevPulse\Shared\MonthRange;
use DevPulse\Shared\RepoFullName;
use DevPulse\Vcs\Author;
use DevPulse\Vcs\PullRequestNumber;
use App\Models\Build;
use App\Models\Group;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

class FailedBuildListQuery
{
    public function __construct(private readonly BuildEventFilter $filter)
    {
    }

    /**
     * 列出指定 group 在指定月份的所有失敗 build（含 commit、PR、author、時間）。
     *
     * @return Collection<int, FailedBuildItem>
     */
    public function run(Group $group, MonthRange $month): Collection
    {
        $query = Build::query()
            ->join('repos', 'repos.id', '=', 'builds.repo_id')
            ->whereIn('builds.repo_id', $group->repoIds())
            ->where('builds.started_at', '>=', $month->start)
            ->where('builds.started_at', '<', $month->end)
            ->where('builds.is_failure', true);

        $this->filter->apply($query);

        /** @var \Illuminate\Support\Collection<int, object{repo_full_name: string, external_id: string, commit_sha: string, author_account: string|null, pr_number: int|null, status: string, started_at: string}> $rows */
        $rows = $query
            ->select([
                'repos.full_name as repo_full_name',
                'builds.external_id',
                'builds.commit_sha',
                'builds.author_account',
                'builds.pr_number',
                'builds.status',
                'builds.started_at',
            ])
            ->orderBy('builds.started_at')
            ->get();

        return $rows->map(fn ($row) => new FailedBuildItem(
            repoFullName: new RepoFullName($row->repo_full_name),
            externalId: (string)$row->external_id,
            commitSha: new CommitSha((string)$row->commit_sha),
            authorAccount: $row->author_account !== null ? new Author($row->author_account) : null,
            prNumber: $row->pr_number !== null ? new PullRequestNumber((int)$row->pr_number) : null,
            status: (string)$row->status,
            startedAt: CarbonImmutable::parse($row->started_at),
        ));
    }
}
