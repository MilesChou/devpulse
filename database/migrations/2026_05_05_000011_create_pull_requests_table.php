<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('dp_pull_requests', function (Blueprint $table): void {
            $table->id();
            $table->ulid('ulid')->nullable()->unique();
            $table->string('platform', 16)->nullable();
            $table->foreignId('repo_id')->constrained('dp_repos')->cascadeOnDelete();
            $table->integer('number');
            $table->string('author_account', 64);
            $table->string('status', 16);
            $table->integer('additions');
            $table->integer('deletions');
            // 以下為去正規化的維度欄位，目的是讓 Grafana / SQL view 能直接
            // WHERE / GROUP BY。Source of truth 在 VO。
            $table->integer('total_changed_lines');
            $table->string('size_bucket', 8)->nullable();
            $table->boolean('is_draft');
            $table->timestamp('pr_created_at');
            $table->timestamp('ready_at')->nullable();
            $table->timestamp('first_review_at')->nullable();
            $table->timestamp('first_approved_at')->nullable();
            // 去正規化的計算欄位，單位為秒，供 Grafana 直接查詢
            $table->unsignedInteger('time_to_approval')->nullable();
            $table->unsignedInteger('time_to_merge')->nullable();
            $table->timestamp('merged_at')->nullable();
            $table->timestamp('closed_at')->nullable();
            $table->timestamps();

            $table->unique(['repo_id', 'number']);
            $table->index(['repo_id', 'pr_created_at']);
            $table->index(['repo_id', 'ready_at']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('dp_pull_requests');
    }
};
