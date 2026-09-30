-- People: members (display names), the GitHub accounts each one uses,
-- teams of members, and accounts excluded from every metric (bots).
--
-- Accounts are stored normalized: lower-cased, with a trailing "[bot]"
-- removed. GitHub's REST API names a bot "dependabot[bot]" while its
-- GraphQL API names the same bot "dependabot", so normalizing both
-- sides lets one entry match either spelling.
CREATE TABLE members (
    id           CHAR(26) NOT NULL PRIMARY KEY,
    display_name VARCHAR(255) NOT NULL,
    created_at   TIMESTAMP NULL,
    updated_at   TIMESTAMP NULL,
    CONSTRAINT members_display_name_uniq UNIQUE (display_name)
);

CREATE TABLE member_accounts (
    account    VARCHAR(64) NOT NULL PRIMARY KEY,
    member_id  CHAR(26) NOT NULL,
    created_at TIMESTAMP NULL
);

CREATE INDEX member_accounts_member_idx ON member_accounts (member_id);

CREATE TABLE teams (
    id         CHAR(26) NOT NULL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,
    CONSTRAINT teams_name_uniq UNIQUE (name)
);

CREATE TABLE team_members (
    team_id   CHAR(26) NOT NULL,
    member_id CHAR(26) NOT NULL,
    PRIMARY KEY (team_id, member_id)
);

CREATE TABLE excluded_accounts (
    account    VARCHAR(64) NOT NULL PRIMARY KEY,
    created_at TIMESTAMP NULL
);

-- Bots observed on real repos: dependabot and github-actions open PRs;
-- copilot-pull-request-reviewer leaves Copilot code reviews.
INSERT INTO excluded_accounts (account) VALUES ('dependabot');
INSERT INTO excluded_accounts (account) VALUES ('github-actions');
INSERT INTO excluded_accounts (account) VALUES ('copilot-pull-request-reviewer');
