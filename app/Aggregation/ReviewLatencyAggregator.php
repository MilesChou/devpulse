<?php

declare(strict_types=1);

namespace App\Aggregation;

use App\Aggregation\Dto\ReviewLatencyResult;
use App\Models\Group;
use App\Models\PullRequest;
use App\Support\Time\MonthRange;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

final class ReviewLatencyAggregator
{
    public function __construct(private readonly ?CarbonImmutable $clock = null)
    {
    }

    /**
     * 計算指定月份、指定 group 內每個 PR 從 ready_at 到首次 review 的等待時數。
     *
     * draft（ready_at IS NULL）不計入。
     * 月底前未收到 review：以月底（或當下，取較早者）為 lower bound，isLowerBound=true。
     *
     * @param string $month YYYY-MM
     * @return Collection<int, ReviewLatencyResult>
     */
    public function aggregate(Group $group, string $month): Collection
    {
        [$start, $end] = MonthRange::parse($month);

        $groupRepoIds = $group->repos()->pluck('repos.id');

        $now = $this->clock ?? CarbonImmutable::now('UTC');
        $cutoff = $end->isBefore($now) ? $end : $now;

        $prs = PullRequest::query()
            ->whereIn('repo_id', $groupRepoIds)
            ->where('pr_created_at', '>=', $start)
            ->where('pr_created_at', '<', $end)
            ->whereNotNull('ready_at')
            ->whereNotNull('size_bucket')
            ->with('repo')
            ->get();

        return $prs->map(function (PullRequest $pr) use ($cutoff): ReviewLatencyResult {
            $readyAt = $pr->ready_at;
            $firstReviewAt = $pr->first_review_at;

            if ($firstReviewAt !== null) {
                $latencySeconds = $readyAt->diffInSeconds($firstReviewAt);
                $isLowerBound = false;
            } else {
                $latencySeconds = $readyAt->diffInSeconds($cutoff);
                $isLowerBound = true;
            }

            return new ReviewLatencyResult(
                repoFullName: $pr->repo->full_name,
                prNumber: $pr->number,
                authorAccount: $pr->author_account,
                sizeBucket: $pr->size_bucket,
                latencyHours: max(0.0, $latencySeconds / 3600.0),
                isLowerBound: $isLowerBound,
            );
        });
    }
}
