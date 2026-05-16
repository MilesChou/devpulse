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

            $table->unique(['pull_request_id', 'reviewer_account', 'submitted_at']);
            $table->index(['pull_request_id', 'state']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('dp_pull_request_reviews');
    }
};
