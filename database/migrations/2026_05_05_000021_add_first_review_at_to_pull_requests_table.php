<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::table('pull_requests', function (Blueprint $table): void {
            $table->timestamp('first_review_at')->nullable()->after('ready_at');
            $table->string('size_bucket', 8)->nullable()->after('total_changed_lines');
            $table->index(['repo_id', 'ready_at']);
        });
    }

    public function down(): void
    {
        Schema::table('pull_requests', function (Blueprint $table): void {
            $table->dropIndex(['repo_id', 'ready_at']);
            $table->dropColumn('size_bucket');
            $table->dropColumn('first_review_at');
        });
    }
};
