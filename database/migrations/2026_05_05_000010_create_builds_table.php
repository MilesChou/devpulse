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
            $table->string('provider', 32);
            $table->string('external_id', 64);
            $table->string('commit_sha', 64);
            $table->string('status', 32);
            $table->string('event_type', 32);
            $table->string('branch', 255)->nullable();
            $table->boolean('is_post_merge');
            $table->boolean('is_pull_request');
            $table->boolean('is_deploy_event');
            $table->boolean('is_failure');
            $table->timestamp('started_at');
            $table->integer('duration_seconds')->nullable();
            $table->json('raw_payload');
            $table->timestamps();

            $table->unique(['provider', 'external_id']);
            $table->index(['repo_id', 'started_at']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('builds');
    }
};
