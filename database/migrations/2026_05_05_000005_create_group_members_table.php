<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('dp_groups_members', function (Blueprint $table): void {
            $table->foreignId('group_id')->constrained('dp_groups')->cascadeOnDelete();
            $table->foreignId('member_id')->constrained('dp_members')->cascadeOnDelete();
            $table->primary(['group_id', 'member_id']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('dp_groups_members');
    }
};
