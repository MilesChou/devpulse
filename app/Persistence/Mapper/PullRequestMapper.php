<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use DevPulse\Vcs\PullRequest;

final class PullRequestMapper
{
    /**
     * @return array<string, mixed>
     */
    public function toAttributes(PullRequest $vo): array
    {
        return [
            'id' => (string)$vo->id,
            'platform' => $vo->platform->value,
            'repo_id' => $vo->repoId->toString(),
            'number' => $vo->number->toInt(),
            'author_account' => (string)$vo->author,
            'status' => $vo->status()->value,
            'additions' => $vo->changes()->additions,
            'deletions' => $vo->changes()->deletions,
            'total_changed_lines' => $vo->changes()->total(),
            'is_draft' => $vo->isDraft(),
            'pr_created_at' => $vo->createdAt,
            'ready_at' => $vo->readyAt(),
            'merged_at' => $vo->status()->isMerged() ? $vo->closedAt() : null,
            'closed_at' => $vo->closedAt(),
        ];
    }
}
