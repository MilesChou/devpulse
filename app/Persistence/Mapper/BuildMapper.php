<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use App\Domain\Ci\BuildSummary;
use App\Models\Build;

final class BuildMapper
{
    /**
     * 把 BuildSummary VO 轉成 Eloquent attribute array（給 firstOrCreate / upsert 用）。
     *
     * @param array<string, mixed> $rawPayload Provider 原始 JSON（持久化用）
     * @return array<string, mixed>
     */
    public function toAttributes(BuildSummary $vo, int $repoId, array $rawPayload): array
    {
        return [
            'repo_id' => $repoId,
            'provider' => $vo->provider->value,
            'external_id' => $vo->externalId,
            'commit_sha' => $vo->commitSha,
            'author_account' => $vo->authorAccount,
            'pr_number' => $vo->prNumber,
            'status' => $vo->status->value,
            'event_type' => $vo->eventType,
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

    /**
     * 從 Eloquent Build model 還原為 BuildSummary VO。
     *
     * repo_full_name 由 caller 提供（避免 N+1 lazy load relation）。
     */
    public function toVo(Build $model, string $repoFullName): BuildSummary
    {
        return new BuildSummary(
            provider: $model->provider,
            externalId: $model->external_id,
            repoFullName: $repoFullName,
            commitSha: $model->commit_sha,
            authorAccount: $model->author_account,
            prNumber: $model->pr_number,
            status: $model->status,
            eventType: $model->event_type,
            branch: $model->branch,
            startedAt: $model->started_at,
            durationSeconds: $model->duration_seconds,
        );
    }
}
