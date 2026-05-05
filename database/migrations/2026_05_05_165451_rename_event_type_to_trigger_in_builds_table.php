<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('builds', function (Blueprint $table): void {
            $table->renameColumn('event_type', 'trigger');
        });
    }

    public function down(): void
    {
        Schema::table('builds', function (Blueprint $table): void {
            $table->renameColumn('trigger', 'event_type');
        });
    }
};
