<?php

use Illuminate\Database\Migrations\Migration;
use Staudenmeir\LaravelMigrationViews\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Extends dpv_pull_requests with pre-computed latency columns.
        // Uses NOW() as fallback for open PRs (instead of Grafana's $__timeTo()).
        Schema::createOrReplaceView('dpv_pr_latency', <<<'SQL'
            SELECT
                group_slug, member, repo,
                id, number, author_account, size_bucket,
                pr_created_at, ready_at,
                first_review_at, merged_at, closed_at,
                EXTRACT(EPOCH FROM (COALESCE(first_review_at, merged_at, closed_at, NOW()) - ready_at)) / 3600.0 AS lat_h,
                EXTRACT(EPOCH FROM (first_review_at - ready_at)) / 3600.0 AS pickup_h,
                time_to_approval / 3600.0 AS approval_h,
                time_to_merge   / 3600.0 AS merge_h,
                (first_review_at IS NULL) AS unreviewed
            FROM dpv_pull_requests
            WHERE ready_at IS NOT NULL AND size_bucket IS NOT NULL
        SQL);

        Schema::createOrReplaceView('dpv_pr_latency_weekly', <<<'SQL'
            SELECT
                group_slug,
                size_bucket,
                date_trunc('week', pr_created_at) AS week,
                percentile_cont(0.5) WITHIN GROUP (ORDER BY lat_h) AS median_h
            FROM dpv_pr_latency
            GROUP BY group_slug, size_bucket, date_trunc('week', pr_created_at)
        SQL);

        Schema::createOrReplaceView('dpv_build_duration_daily', <<<'SQL'
            SELECT
                group_slug,
                repo,
                DATE(started_at) AS day,
                percentile_cont(0.5) WITHIN GROUP (ORDER BY duration_seconds) / 60.0 AS median_min
            FROM dpv_builds
            WHERE status = 'PASSED'
              AND duration_seconds IS NOT NULL
              AND NOT is_post_merge
              AND NOT is_deploy_event
            GROUP BY group_slug, repo, DATE(started_at)
        SQL);

        Schema::createOrReplaceView('dpv_failure_rate_weekly', <<<'SQL'
            SELECT
                group_slug,
                member,
                date_trunc('week', started_at) AS week,
                SUM(CASE WHEN status <> 'CANCELED' THEN 1 ELSE 0 END) AS denom,
                SUM(CASE WHEN is_failure AND status <> 'CANCELED' THEN 1 ELSE 0 END) AS fails
            FROM dpv_builds
            WHERE NOT is_post_merge AND NOT is_deploy_event
            GROUP BY group_slug, member, date_trunc('week', started_at)
        SQL);
    }

    public function down(): void
    {
        Schema::dropViewIfExists('dpv_failure_rate_weekly');
        Schema::dropViewIfExists('dpv_build_duration_daily');
        Schema::dropViewIfExists('dpv_pr_latency_weekly');
        Schema::dropViewIfExists('dpv_pr_latency');
    }
};
