<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('repos', function (Blueprint $table): void {
            $table->id();
            $table->string('full_name', 255)->unique();
            $table->string('ci_provider', 32)->default('travis');
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('repos');
    }
};
