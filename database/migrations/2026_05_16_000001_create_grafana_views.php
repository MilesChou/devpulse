<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\DB;

return new class extends Migration
{
    public function up(): void
    {
        DB::statement('DROP VIEW IF EXISTS dpv_reviews');
        DB::statement('DROP VIEW IF EXISTS dpv_pull_requests');
        DB::statement('DROP VIEW IF EXISTS dpv_builds');

        DB::statement('
            CREATE VIEW dpv_builds AS
            SELECT
                g.slug           AS group_slug,
                m.display_name   AS member,
                r.full_name      AS repo,
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
            JOIN dp_repos r           ON r.id = b.repo_id
            JOIN dp_members m         ON m.github_account = b.author_account
            JOIN dp_groups_repos gr   ON gr.repo_id = r.id
            JOIN dp_groups_members gm ON gm.group_id = gr.group_id AND gm.member_id = m.id
            JOIN dp_groups g          ON g.id = gr.group_id
        ');

        DB::statement('
            CREATE VIEW dpv_pull_requests AS
            SELECT
                g.slug           AS group_slug,
                m.display_name   AS member,
                r.full_name      AS repo,
                pr.id,
                pr.number,
                pr.author_account,
                pr.size_bucket,
                pr.pr_created_at,
                pr.ready_at,
                pr.first_review_at,
                pr.time_to_approval,
                pr.time_to_merge,
                pr.merged_at,
                pr.closed_at
            FROM dp_pull_requests pr
            JOIN dp_repos r           ON r.id = pr.repo_id
            JOIN dp_members m         ON m.github_account = pr.author_account
            JOIN dp_groups_repos gr   ON gr.repo_id = r.id
            JOIN dp_groups_members gm ON gm.group_id = gr.group_id AND gm.member_id = m.id
            JOIN dp_groups g          ON g.id = gr.group_id
        ');

        DB::statement('
            CREATE VIEW dpv_reviews AS
            SELECT
                g.slug                  AS group_slug,
                reviewer_m.display_name AS reviewer,
                author_m.display_name   AS author,
                prr.reviewer_account,
                pr.author_account,
                pr.pr_created_at,
                pr.id                   AS pr_id,
                prr.state
            FROM dp_pull_request_reviews prr
            JOIN dp_pull_requests pr       ON pr.id = prr.pull_request_id
            JOIN dp_repos r                ON r.id = pr.repo_id
            JOIN dp_groups_repos gr        ON gr.repo_id = r.id
            JOIN dp_groups g               ON g.id = gr.group_id
            JOIN dp_members reviewer_m     ON reviewer_m.github_account = prr.reviewer_account
            JOIN dp_groups_members rev_gm  ON rev_gm.member_id = reviewer_m.id AND rev_gm.group_id = gr.group_id
            JOIN dp_members author_m       ON author_m.github_account = pr.author_account
            JOIN dp_groups_members auth_gm ON auth_gm.member_id = author_m.id AND auth_gm.group_id = gr.group_id
        ');
    }

    public function down(): void
    {
        DB::statement('DROP VIEW IF EXISTS dpv_reviews');
        DB::statement('DROP VIEW IF EXISTS dpv_pull_requests');
        DB::statement('DROP VIEW IF EXISTS dpv_builds');
    }
};
