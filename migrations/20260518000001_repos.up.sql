CREATE TABLE repos (
    id              CHAR(26) NOT NULL PRIMARY KEY,
    provider        VARCHAR(64) NOT NULL,
    owner           VARCHAR(255) NOT NULL,
    repo_name       VARCHAR(255) NOT NULL,
    url             VARCHAR(500) NOT NULL,
    description     TEXT NULL,
    default_branch  VARCHAR(255) NOT NULL DEFAULT '',
    disabled        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP NULL,
    updated_at      TIMESTAMP NULL,
    CONSTRAINT repos_provider_owner_repo_name_uniq UNIQUE (provider, owner, repo_name)
);

