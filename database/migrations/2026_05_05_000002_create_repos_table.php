<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Query\Expression;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::create('dp_repos', function (Blueprint $table): void {
            $table->ulid('id')->primary();
            $table->string('slug', 64)->unique();
            $table->string('name', 255);
            $table->string('type', 256);
            $table->string('url', 500);
            // human_signals 用於 classifier：每筆 { "category": "lint", "pattern": "PHPCS:" }
            // classifier 對失敗 build 的 log 做字串比對，把 build 歸類為 human / infra。
            // MySQL 8 要求 JSON column 的預設值必須用 expression default (parenthesized)。
            $table->json('human_signals')->default(new Expression("('[]')"));
            $table->timestamps();

            $table->unique(['type', 'name']);
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('dp_repos');
    }
};
