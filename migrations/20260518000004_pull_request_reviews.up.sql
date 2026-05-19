CREATE TABLE pull_request_reviews (
    id               CHAR(26) NOT NULL PRIMARY KEY,
    pull_request_id  CHAR(26) NOT NULL,
    reviewer_account VARCHAR(64) NOT NULL,
    state            VARCHAR(32) NOT NULL,
    submitted_at     TIMESTAMP NOT NULL,
    created_at       TIMESTAMP NULL,
    updated_at       TIMESTAMP NULL,
    CONSTRAINT pull_request_reviews_pr_reviewer_submitted_uniq UNIQUE (pull_request_id, reviewer_account, submitted_at)
);

CREATE INDEX pull_request_reviews_pr_state_idx ON pull_request_reviews (pull_request_id, state);

