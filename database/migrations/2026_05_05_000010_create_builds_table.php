<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('builds', function (Blueprint $table): void {
            $table->id();
            $table->foreignId('repo_id')->constrained('repos')->cascadeOnDelete();
            $table->string('external_id', 64);
            $table->string('commit_sha', 64);
            $table->string('author_account', 64)->nullable();
            $table->integer('pr_number')->nullable();
            $table->string('status', 32);
            $table->string('trigger', 32);
            $table->string('branch', 255)->nullable();
            // 以下 is_* 為去正規化的維度欄位，目的是讓 Grafana / SQL view 能直接
            // WHERE / GROUP BY，避免在 query 端 evaluate VO 業務規則。Source of truth
            // 仍在 Build VO（fromXxxRaw / isPostMerge 等 method）。
            $table->boolean('is_post_merge');
            $table->boolean('is_pull_request');
            $table->boolean('is_deploy_event');
            $table->boolean('is_failure');
            $table->timestamp('started_at');
            $table->integer('duration_seconds')->nullable();
            // 保留 provider 原始 JSON 以便日後新增分析欄位時不需重打 API。
            // 注意：Travis build 含 jobs 可能 50KB+，1000 筆 = 50MB；caller 應在
            // 寫入前剝掉確定不需要的巢狀欄位（例如 build.jobs.config）。
            $table->json('raw_payload');
            $table->timestamps();

            $table->unique(['repo_id', 'external_id']);
            $table->index(['repo_id', 'started_at']);
            $table->index(['repo_id', 'author_account', 'started_at']);
            $table->index(['repo_id', 'pr_number']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('builds');
    }
};
