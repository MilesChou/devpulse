<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\DB;
use Staudenmeir\LaravelMigrationViews\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // SQLite is only used in tests and never serves Grafana; skip view
        // creation there to keep RefreshDatabase tests green.
        if (DB::connection()->getDriverName() === 'sqlite') {
            return;
        }

        // MySQL 8 lacks percentile_cont(), so weekly/daily medians are computed
        // via window functions in derived tables (AVG over the two middle rows).
        // Week buckets follow Monday-start ISO weeks via WEEKDAY().
        Schema::createOrReplaceView('dpv_pr_latency', <<<'SQL'
            SELECT
                repo,
                id, number, author_account,
                pr_created_at, ready_at,
                first_review_at, merged_at, closed_at,
                TIMESTAMPDIFF(SECOND, ready_at, COALESCE(first_review_at, merged_at, closed_at, NOW())) / 3600.0 AS lat_h,
                TIMESTAMPDIFF(SECOND, ready_at, first_review_at) / 3600.0 AS pickup_h,
                time_to_approval / 3600.0 AS approval_h,
                time_to_merge   / 3600.0 AS merge_h,
                (first_review_at IS NULL) AS unreviewed
            FROM dpv_pull_requests
            WHERE ready_at IS NOT NULL
        SQL);

        Schema::createOrReplaceView('dpv_pr_latency_weekly', <<<'SQL'
            SELECT
                week,
                AVG(lat_h) AS median_h
            FROM (
                SELECT
                    DATE_SUB(DATE(pr_created_at), INTERVAL WEEKDAY(pr_created_at) DAY) AS week,
                    lat_h,
                    ROW_NUMBER() OVER (
                        PARTITION BY DATE_SUB(DATE(pr_created_at), INTERVAL WEEKDAY(pr_created_at) DAY)
                        ORDER BY lat_h
                    ) AS rn,
                    COUNT(*) OVER (
                        PARTITION BY DATE_SUB(DATE(pr_created_at), INTERVAL WEEKDAY(pr_created_at) DAY)
                    ) AS cnt
                FROM dpv_pr_latency
            ) t
            WHERE rn IN (FLOOR((cnt + 1) / 2), FLOOR(cnt / 2) + 1)
            GROUP BY week
        SQL);

        Schema::createOrReplaceView('dpv_build_duration_daily', <<<'SQL'
            SELECT
                repo,
                day,
                AVG(duration_seconds) / 60.0 AS median_min
            FROM (
                SELECT
                    repo,
                    DATE(started_at) AS day,
                    duration_seconds,
                    ROW_NUMBER() OVER (
                        PARTITION BY repo, DATE(started_at)
                        ORDER BY duration_seconds
                    ) AS rn,
                    COUNT(*) OVER (
                        PARTITION BY repo, DATE(started_at)
                    ) AS cnt
                FROM dpv_builds
                WHERE status = 'PASSED'
                  AND duration_seconds IS NOT NULL
                  AND NOT is_post_merge
                  AND NOT is_deploy_event
            ) t
            WHERE rn IN (FLOOR((cnt + 1) / 2), FLOOR(cnt / 2) + 1)
            GROUP BY repo, day
        SQL);

        Schema::createOrReplaceView('dpv_failure_rate_weekly', <<<'SQL'
            SELECT
                repo,
                DATE_SUB(DATE(started_at), INTERVAL WEEKDAY(started_at) DAY) AS week,
                SUM(CASE WHEN status <> 'CANCELED' THEN 1 ELSE 0 END) AS denom,
                SUM(CASE WHEN is_failure AND status <> 'CANCELED' THEN 1 ELSE 0 END) AS fails
            FROM dpv_builds
            WHERE NOT is_post_merge AND NOT is_deploy_event
            GROUP BY repo,
                     DATE_SUB(DATE(started_at), INTERVAL WEEKDAY(started_at) DAY)
        SQL);
    }

    public function down(): void
    {
        if (DB::connection()->getDriverName() === 'sqlite') {
            return;
        }

        Schema::dropViewIfExists('dpv_failure_rate_weekly');
        Schema::dropViewIfExists('dpv_build_duration_daily');
        Schema::dropViewIfExists('dpv_pr_latency_weekly');
        Schema::dropViewIfExists('dpv_pr_latency');
    }
};
