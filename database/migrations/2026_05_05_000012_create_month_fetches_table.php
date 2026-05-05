<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

/**
 * 記錄每個 (repo, dataset, month) 組合的撈取狀態，讓「已過月份不重撈」邏輯能查。
 *
 * dataset 區分 builds / pull_requests（不同 endpoint 各自獨立）。
 * status: complete = 該月完整撈過、partial = 撈到一半（例如 rate limit 中斷）
 */
return new class () extends Migration {
    public function up(): void
    {
        Schema::create('month_fetches', function (Blueprint $table): void {
            $table->id();
            $table->foreignId('repo_id')->constrained('repos')->cascadeOnDelete();
            $table->string('dataset', 32);
            $table->string('month', 7);
            $table->string('status', 16);
            $table->timestamp('fetched_at');
            $table->timestamps();

            $table->unique(['repo_id', 'dataset', 'month']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('month_fetches');
    }
};
