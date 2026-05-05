<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('pull_requests', function (Blueprint $table): void {
            $table->id();
            $table->foreignId('repo_id')->constrained('repos')->cascadeOnDelete();
            $table->integer('number');
            $table->string('author_account', 64);
            $table->string('status', 16);
            $table->integer('additions');
            $table->integer('deletions');
            $table->integer('total_changed_lines');
            $table->boolean('is_draft');
            $table->timestamp('pr_created_at');
            $table->timestamp('ready_at')->nullable();
            $table->timestamp('merged_at')->nullable();
            $table->timestamp('closed_at')->nullable();
            $table->json('raw_payload');
            $table->timestamps();

            $table->unique(['repo_id', 'number']);
            $table->index(['repo_id', 'pr_created_at']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('pull_requests');
    }
};
