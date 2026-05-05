<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::table('builds', function (Blueprint $table): void {
            $table->integer('pr_number')->nullable()->after('author_account');
            $table->index(['repo_id', 'pr_number']);
        });
    }

    public function down(): void
    {
        Schema::table('builds', function (Blueprint $table): void {
            $table->dropIndex(['repo_id', 'pr_number']);
            $table->dropColumn('pr_number');
        });
    }
};
