CREATE TABLE repos (
    id            CHAR(26) NOT NULL PRIMARY KEY,
    slug          VARCHAR(64) NOT NULL UNIQUE,
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(64) NOT NULL,
    url           VARCHAR(500) NOT NULL,
    created_at    TIMESTAMP NULL,
    updated_at    TIMESTAMP NULL,
    CONSTRAINT repos_type_name_uniq UNIQUE (type, name)
);

