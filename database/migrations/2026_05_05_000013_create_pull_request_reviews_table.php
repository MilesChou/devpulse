<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('dp_pull_request_reviews', function (Blueprint $table): void {
            $table->id();
            $table->foreignId('pull_request_id')->constrained('dp_pull_requests')->cascadeOnDelete();
            $table->string('reviewer_account', 64);
            $table->string('state', 32);
            $table->timestamp('submitted_at');
            $table->timestamps();

            // Explicit names to stay within MySQL's 64-char identifier limit
            // (Laravel's auto-generated names would be ~76 chars for the unique).
            $table->unique(['pull_request_id', 'reviewer_account', 'submitted_at'], 'dp_pull_request_reviews_pr_reviewer_submitted_unique');
            $table->index(['pull_request_id', 'state'], 'dp_pull_request_reviews_pr_state_index');
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('dp_pull_request_reviews');
    }
};
