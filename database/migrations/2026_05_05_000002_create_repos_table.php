<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('dp_repos', function (Blueprint $table): void {
            $table->id();
            $table->string('full_name', 255)->unique();
            // human_signals 用於 classifier：每筆 { "category": "lint", "pattern": "PHPCS:" }
            // classifier 對失敗 build 的 log 做字串比對，把 build 歸類為 human / infra。
            $table->json('human_signals')->default(json_encode([]));
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('dp_repos');
    }
};
