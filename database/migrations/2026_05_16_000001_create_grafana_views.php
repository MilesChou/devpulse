<?php

use Illuminate\Database\Migrations\Migration;
use Staudenmeir\LaravelMigrationViews\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::createOrReplaceView('dpv_builds', '
            SELECT
                r.name           AS repo,
                b.id,
                b.external_id,
                b.author_account,
                b.pr_number,
                b.status,
                b.branch,
                b.is_post_merge,
                b.is_pull_request,
                b.is_deploy_event,
                b.is_failure,
                b.started_at,
                b.duration_seconds
            FROM dp_builds b
            JOIN dp_repos r ON r.id = b.repo_id
        ');

        Schema::createOrReplaceView('dpv_pull_requests', '
            SELECT
                r.name           AS repo,
                pr.id,
                pr.number,
                pr.author_account,
                pr.pr_created_at,
                pr.ready_at,
                pr.first_review_at,
                pr.time_to_approval,
                pr.time_to_merge,
                pr.merged_at,
                pr.closed_at
            FROM dp_pull_requests pr
            JOIN dp_repos r ON r.id = pr.repo_id
        ');

        Schema::createOrReplaceView('dpv_reviews', '
            SELECT
                r.name                  AS repo,
                prr.reviewer_account,
                pr.author_account,
                pr.pr_created_at,
                pr.id                   AS pr_id,
                prr.state
            FROM dp_pull_request_reviews prr
            JOIN dp_pull_requests pr ON pr.id = prr.pull_request_id
            JOIN dp_repos r          ON r.id = pr.repo_id
        ');
    }

    public function down(): void
    {
        Schema::dropViewIfExists('dpv_reviews');
        Schema::dropViewIfExists('dpv_pull_requests');
        Schema::dropViewIfExists('dpv_builds');
    }
};
