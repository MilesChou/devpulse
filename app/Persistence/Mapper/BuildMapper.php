<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use App\Domain\Ci\Build;

final class BuildMapper
{
    /**
     * 把 Build VO 轉成 Eloquent attribute array（給 firstOrCreate / upsert 用）。
     *
     * @param array<string, mixed> $rawPayload Provider 原始 JSON（持久化用）
     * @return array<string, mixed>
     */
    public function toAttributes(Build $vo, int $repoId, array $rawPayload): array
    {
        return [
            'repo_id' => $repoId,
            'external_id' => $vo->externalId,
            'commit_sha' => (string)$vo->commitSha,
            'author_account' => $vo->authorAccount,
            'pr_number' => $vo->prNumber,
            'status' => $vo->status->name,
            'trigger' => $vo->trigger->name,
            'branch' => $vo->branch,
            'is_post_merge' => $vo->isPostMerge(),
            'is_pull_request' => $vo->isPullRequest(),
            'is_deploy_event' => $vo->isDeployEvent(),
            'is_failure' => $vo->isFailure(),
            'started_at' => $vo->startedAt,
            'duration_seconds' => $vo->durationSeconds,
            'raw_payload' => $rawPayload,
        ];
    }
}
