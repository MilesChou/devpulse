DROP TABLE incidents;

ALTER TABLE repos DROP COLUMN pr_updated_watermark;
ALTER TABLE repos DROP COLUMN hotfix_label;
ALTER TABLE repos DROP COLUMN incident_label;

DROP INDEX pull_requests_repo_head_ref_idx;
DROP INDEX pull_requests_repo_merged_idx;

ALTER TABLE pull_requests DROP COLUMN reverts_number;
ALTER TABLE pull_requests DROP COLUMN first_commit_at;
ALTER TABLE pull_requests DROP COLUMN merge_commit_sha;
ALTER TABLE pull_requests DROP COLUMN head_ref;
ALTER TABLE pull_requests DROP COLUMN base_ref;
ALTER TABLE pull_requests DROP COLUMN labels;
ALTER TABLE pull_requests DROP COLUMN title;
