<?php

declare(strict_types=1);

namespace App\Persistence\Mapper;

use App\Domain\Ci\BuildStatus;
use App\Domain\Ci\BuildSummary;
use App\Domain\Ci\BuildTrigger;
use App\Domain\Shared\CommitSha;
use App\Domain\Shared\RepoFullName;
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
            'external_id' => $vo->externalId,
            'commit_sha' => (string) $vo->commitSha,
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

    /**
     * 從 Eloquent Build model 還原為 BuildSummary VO。
     *
     * repo_full_name 由 caller 提供（避免 N+1 lazy load relation）。
     */
    public function toVo(Build $model, RepoFullName $repoFullName): BuildSummary
    {
        return new BuildSummary(
            externalId: $model->external_id,
            repoFullName: $repoFullName,
            commitSha: new CommitSha($model->commit_sha),
            authorAccount: $model->author_account,
            prNumber: $model->pr_number,
            status: $this->resolveStatus($model->status),
            trigger: $this->resolveTrigger($model->trigger),
            branch: $model->branch,
            startedAt: $model->started_at,
            durationSeconds: $model->duration_seconds,
        );
    }

    private function resolveTrigger(string $value): BuildTrigger
    {
        return match ($value) {
            BuildTrigger::PULL_REQUEST->name => BuildTrigger::PULL_REQUEST,
            BuildTrigger::POST_MERGE->name => BuildTrigger::POST_MERGE,
            BuildTrigger::SCHEDULED->name => BuildTrigger::SCHEDULED,
            BuildTrigger::MANUAL->name => BuildTrigger::MANUAL,
            default => BuildTrigger::PULL_REQUEST,
        };
    }

    private function resolveStatus(string $value): BuildStatus
    {
        return match ($value) {
            BuildStatus::PASSED->name => BuildStatus::PASSED,
            BuildStatus::FAILED->name, 'errored' => BuildStatus::FAILED,
            BuildStatus::CANCELED->name => BuildStatus::CANCELED,
            default => BuildStatus::IN_PROGRESS,
        };
    }
}
