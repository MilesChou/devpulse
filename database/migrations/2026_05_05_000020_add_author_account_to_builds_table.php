<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::table('builds', function (Blueprint $table): void {
            $table->string('author_account', 64)->nullable()->after('commit_sha');
            $table->index(['repo_id', 'author_account', 'started_at']);
        });
    }

    public function down(): void
    {
        Schema::table('builds', function (Blueprint $table): void {
            $table->dropIndex(['repo_id', 'author_account', 'started_at']);
            $table->dropColumn('author_account');
        });
    }
};
