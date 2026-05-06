<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use App\Domain\Vcs\PullRequest;
use Carbon\CarbonImmutable;

final class PullRequestMapper
{
    /**
     * @return array<string, mixed>
     */
    public function toAttributes(
        PullRequest $vo,
        ?CarbonImmutable $firstReviewAt = null,
        ?string $sizeBucket = null,
    ): array {
        return [
            'repo_id' => $vo->repoId->toInt(),
            'number' => $vo->number->toInt(),
            'author_account' => $vo->author->toString(),
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
        ];
    }
}

