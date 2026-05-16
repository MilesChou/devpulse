<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('dp_groups_repos', function (Blueprint $table) {
            $table->index('repo_id');
        });

        Schema::table('dp_groups_members', function (Blueprint $table) {
            $table->index('member_id');
        });
    }

    public function down(): void
    {
        Schema::table('dp_groups_repos', function (Blueprint $table) {
            $table->dropIndex(['repo_id']);
        });

        Schema::table('dp_groups_members', function (Blueprint $table) {
            $table->dropIndex(['member_id']);
        });
    }
};
