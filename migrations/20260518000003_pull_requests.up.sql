CREATE TABLE pull_requests (
    id                  CHAR(26) NOT NULL PRIMARY KEY,
    platform            VARCHAR(16) NULL,
    repo_id             CHAR(26) NOT NULL,
    number              INTEGER NOT NULL,
    author_account      VARCHAR(64) NOT NULL,
    status              VARCHAR(16) NOT NULL,
    additions           INTEGER NOT NULL,
    deletions           INTEGER NOT NULL,
    total_changed_lines INTEGER NOT NULL,
    size_bucket         VARCHAR(8) NULL,
    is_draft            BOOLEAN NOT NULL,
    pr_created_at       TIMESTAMP NOT NULL,
    ready_at            TIMESTAMP NULL,
    first_review_at     TIMESTAMP NULL,
    first_approved_at   TIMESTAMP NULL,
    time_to_approval    INTEGER NULL,
    time_to_merge       INTEGER NULL,
    merged_at           TIMESTAMP NULL,
    closed_at           TIMESTAMP NULL,
    created_at          TIMESTAMP NULL,
    updated_at          TIMESTAMP NULL,
    CONSTRAINT pull_requests_repo_number_uniq UNIQUE (repo_id, number),
    CONSTRAINT pull_requests_time_to_approval_nonneg CHECK (time_to_approval IS NULL OR time_to_approval >= 0),
    CONSTRAINT pull_requests_time_to_merge_nonneg    CHECK (time_to_merge    IS NULL OR time_to_merge    >= 0)
);

CREATE INDEX pull_requests_repo_created_idx ON pull_requests (repo_id, pr_created_at);
CREATE INDEX pull_requests_repo_ready_idx   ON pull_requests (repo_id, ready_at);

