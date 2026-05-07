<?php

declare(strict_types=1);

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class () extends Migration {
    public function up(): void
    {
        Schema::table('pull_requests', function (Blueprint $table): void {
            $table->timestamp('first_approved_at')->nullable()->after('first_review_at');
            // 去正規化的計算欄位，單位為秒，供 Grafana 直接查詢
            $table->unsignedInteger('time_to_approval')->nullable()->after('first_approved_at');
            $table->unsignedInteger('time_to_merge')->nullable()->after('time_to_approval');
        });
    }

    public function down(): void
    {
        Schema::table('pull_requests', function (Blueprint $table): void {
            $table->dropColumn(['first_approved_at', 'time_to_approval', 'time_to_merge']);
        });
    }
};
