CREATE TABLE builds (
    id               CHAR(26) NOT NULL PRIMARY KEY,
    repo_id          CHAR(26) NOT NULL,
    external_id      VARCHAR(64) NOT NULL,
    commit_sha       VARCHAR(64) NOT NULL,
    author_account   VARCHAR(64) NULL,
    pr_number        INTEGER NULL,
    status           VARCHAR(32) NOT NULL,
    trigger          VARCHAR(32) NOT NULL,
    branch           VARCHAR(255) NULL,
    is_post_merge    BOOLEAN NOT NULL,
    is_pull_request  BOOLEAN NOT NULL,
    is_deploy_event  BOOLEAN NOT NULL,
    is_failure       BOOLEAN NOT NULL,
    started_at       TIMESTAMP NOT NULL,
    duration_seconds INTEGER NULL,
    raw_payload      TEXT NOT NULL,
    created_at       TIMESTAMP NULL,
    updated_at       TIMESTAMP NULL,
    CONSTRAINT builds_repo_external_uniq UNIQUE (repo_id, external_id)
);

CREATE INDEX builds_repo_started_idx           ON builds (repo_id, started_at);
CREATE INDEX builds_repo_author_started_idx    ON builds (repo_id, author_account, started_at);
CREATE INDEX builds_repo_pr_idx                ON builds (repo_id, pr_number);

