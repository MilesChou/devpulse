<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use App\Domain\Vcs\PullRequestSummary;
use Carbon\CarbonImmutable;

final class PullRequestMapper
{
    /**
     * @param array<string, mixed> $rawPayload
     * @return array<string, mixed>
     */
    public function toAttributes(
        PullRequestSummary $vo,
        int $repoId,
        array $rawPayload,
        ?CarbonImmutable $firstReviewAt = null,
        ?string $sizeBucket = null,
    ): array {
        return [
            'repo_id' => $repoId,
            'number' => $vo->number,
            'author_account' => $vo->authorAccount,
            'status' => $vo->status->value,
            'additions' => $vo->additions,
            'deletions' => $vo->deletions,
            'total_changed_lines' => $vo->totalChangedLines(),
            'size_bucket' => $sizeBucket,
            'is_draft' => $vo->isDraft(),
            'pr_created_at' => $vo->createdAt,
            'ready_at' => $vo->readyAt,
            'first_review_at' => $firstReviewAt,
            'merged_at' => $vo->mergedAt,
            'closed_at' => $vo->closedAt,
            'raw_payload' => $rawPayload,
        ];
    }
}
