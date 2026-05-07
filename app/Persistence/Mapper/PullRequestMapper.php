<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use DevPulse\Vcs\PullRequest;
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
            'ulid' => (string)$vo->id,
            'platform' => $vo->platform->value,
            'repo_id' => $vo->repoId->toInt(),
            'number' => $vo->number->toInt(),
            'author_account' => (string)$vo->author,
            'status' => $vo->status()->value,
            'additions' => $vo->changes()->additions,
            'deletions' => $vo->changes()->deletions,
            'total_changed_lines' => $vo->changes()->total(),
            'size_bucket' => $sizeBucket,
            'is_draft' => $vo->isDraft(),
            'pr_created_at' => $vo->createdAt,
            'ready_at' => $vo->readyAt(),
            'first_review_at' => $firstReviewAt,
            'merged_at' => $vo->status()->isMerged() ? $vo->closedAt() : null,
            'closed_at' => $vo->closedAt(),
        ];
    }
}
