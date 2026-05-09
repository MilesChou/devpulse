<?php

declare(strict_types=1);

namespace App\Services\Web;

use App\Models\PullRequest;
use App\Support\Statistics;
use Carbon\CarbonImmutable;
use Illuminate\Support\Collection;

final class PrLifecycleQuery
{
    /**
     * 區間內整體 PR Lifecycle 三項 p90（秒）。
     *
     * 對應 Grafana panel 10「PR Lifecycle p90」的 Overall row：
     * 母體為 pr_created_at 落在 [from, to] 且 ready_at 不為 null 的 PR。
     *
     * - Pickup：first_review_at - ready_at
     * - Approval：time_to_approval
     * - Merge：time_to_merge
     *
     * @return array{
     *     pickup_p90_seconds: int|null,
     *     approval_p90_seconds: int|null,
     *     merge_p90_seconds: int|null,
     *     pr_count: int,
     * }
     */
    public function overallP90(CarbonImmutable $from, CarbonImmutable $to): array
    {
        /** @var Collection<int, PullRequest> $prs */
        $prs = PullRequest::query()
            ->whereBetween('pr_created_at', [$from->startOfDay(), $to->endOfDay()])
            ->whereNotNull('ready_at')
            ->get(['ready_at', 'first_review_at', 'time_to_approval', 'time_to_merge']);

        $pickup = [];
        $approval = [];
        $merge = [];

        foreach ($prs as $pr) {
            $readyAt = $pr->ready_at;
            $firstReviewAt = $pr->first_review_at;
            if ($readyAt !== null && $firstReviewAt !== null) {
                $pickup[] = max(0, $firstReviewAt->getTimestamp() - $readyAt->getTimestamp());
            }
            if ($pr->time_to_approval !== null) {
                $approval[] = (int)$pr->time_to_approval;
            }
            if ($pr->time_to_merge !== null) {
                $merge[] = (int)$pr->time_to_merge;
            }
        }

        return [
            'pickup_p90_seconds' => $this->p90Seconds($pickup),
            'approval_p90_seconds' => $this->p90Seconds($approval),
            'merge_p90_seconds' => $this->p90Seconds($merge),
            'pr_count' => $prs->count(),
        ];
    }

    /**
     * @param list<int> $values
     */
    private function p90Seconds(array $values): ?int
    {
        $p = Statistics::percentile($values, 0.9);
        return $p === null ? null : (int)round($p);
    }
}
