<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('group_repos', function (Blueprint $table): void {
            $table->foreignId('group_id')->constrained('groups')->cascadeOnDelete();
            $table->foreignId('repo_id')->constrained('repos')->cascadeOnDelete();
            $table->primary(['group_id', 'repo_id']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('group_repos');
    }
};
