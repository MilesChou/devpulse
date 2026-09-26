-- DORA metrics: deployment / remediation facts on PRs, per-repo labels,
-- and mirrored incident issues. One column per ALTER statement keeps the
-- file portable across SQLite, PostgreSQL and MySQL.
ALTER TABLE pull_requests ADD COLUMN title VARCHAR(1024) NULL;
ALTER TABLE pull_requests ADD COLUMN labels TEXT NULL;
ALTER TABLE pull_requests ADD COLUMN base_ref VARCHAR(255) NULL;
ALTER TABLE pull_requests ADD COLUMN head_ref VARCHAR(255) NULL;
ALTER TABLE pull_requests ADD COLUMN merge_commit_sha VARCHAR(64) NULL;
ALTER TABLE pull_requests ADD COLUMN first_commit_at TIMESTAMP NULL;
ALTER TABLE pull_requests ADD COLUMN reverts_number INTEGER NULL;

CREATE INDEX pull_requests_repo_merged_idx ON pull_requests (repo_id, merged_at);

ALTER TABLE repos ADD COLUMN incident_label VARCHAR(255) NOT NULL DEFAULT 'incident';
ALTER TABLE repos ADD COLUMN hotfix_label VARCHAR(255) NOT NULL DEFAULT 'hotfix';
-- Upstream updated_at of the most recently updated PR seen by the last
-- fully successful PR refresh. NULL until the first refresh completes.
ALTER TABLE repos ADD COLUMN pr_updated_watermark TIMESTAMP NULL;

CREATE TABLE incidents (
    id          CHAR(26) NOT NULL PRIMARY KEY,
    repo_id     CHAR(26) NOT NULL,
    source      VARCHAR(32) NOT NULL,
    number      INTEGER NOT NULL,
    title       VARCHAR(1024) NOT NULL,
    opened_at   TIMESTAMP NOT NULL,
    resolved_at TIMESTAMP NULL,
    created_at  TIMESTAMP NULL,
    updated_at  TIMESTAMP NULL,
    CONSTRAINT incidents_repo_source_number_uniq UNIQUE (repo_id, source, number)
);

CREATE INDEX incidents_repo_resolved_idx ON incidents (repo_id, resolved_at);
