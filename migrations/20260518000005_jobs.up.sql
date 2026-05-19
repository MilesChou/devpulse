CREATE TABLE jobs (
    id            CHAR(26) NOT NULL PRIMARY KEY,
    kind          VARCHAR(64) NOT NULL,
    payload       TEXT NOT NULL,
    status        VARCHAR(16) NOT NULL,
    attempts      INTEGER NOT NULL,
    max_attempts  INTEGER NOT NULL,
    last_error    TEXT NULL,
    available_at  TIMESTAMP NOT NULL,
    leased_until  TIMESTAMP NULL,
    created_at    TIMESTAMP NOT NULL,
    updated_at    TIMESTAMP NOT NULL
);

CREATE INDEX jobs_status_available_idx ON jobs (status, available_at);
CREATE INDEX jobs_kind_idx             ON jobs (kind);

