<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use App\Domain\Vcs\PullRequestSummary;
use App\Models\PullRequest;

final class PullRequestMapper
{
    /**
     * @param array<string, mixed> $rawPayload
     * @return array<string, mixed>
     */
    public function toAttributes(PullRequestSummary $vo, int $repoId, array $rawPayload): array
    {
        return [
            'repo_id' => $repoId,
            'number' => $vo->number,
            'author_account' => $vo->authorAccount,
            'status' => $vo->status->value,
            'additions' => $vo->additions,
            'deletions' => $vo->deletions,
            'total_changed_lines' => $vo->totalChangedLines(),
            'is_draft' => $vo->isDraft(),
            'pr_created_at' => $vo->createdAt,
            'ready_at' => $vo->readyAt,
            'merged_at' => $vo->mergedAt,
            'closed_at' => $vo->closedAt,
            'raw_payload' => $rawPayload,
        ];
    }

    public function toVo(PullRequest $model, string $repoFullName): PullRequestSummary
    {
        return new PullRequestSummary(
            repoFullName: $repoFullName,
            number: $model->number,
            authorAccount: $model->author_account,
            status: $model->status,
            additions: $model->additions,
            deletions: $model->deletions,
            createdAt: $model->pr_created_at,
            readyAt: $model->ready_at,
            mergedAt: $model->merged_at,
            closedAt: $model->closed_at,
        );
    }
}
