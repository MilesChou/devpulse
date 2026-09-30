## ADDED Requirements

### Requirement: Metrics across all repos

The user MUST be able to fetch the metrics report and the monthly trend across all tracked, enabled repos, for everyone or scoped to a member or team, in the same shape as the per-repo endpoints.

#### Scenario: Cross-repo report

- **WHEN** the client requests `GET /api/v1/metrics?from=2026-09&member=<id>`
- **THEN** the answer is a report with `repo` set to `*`, covering that member's work in every tracked, enabled repo, and `dora` null

#### Scenario: One account

- **WHEN** the client adds `account=<login>` instead of `member` or `team`
- **THEN** the report covers that account's work alone (normalized like member accounts), names it as an `account` scope, and more than one of `member`, `team` and `account` is rejected with 400

#### Scenario: Cross-repo trend

- **WHEN** the client requests `GET /api/v1/metrics/monthly?from=2025-10&to=2026-10`
- **THEN** the answer has one cross-repo report per month, and a range over 120 months is rejected with 400 as for a single repo

### Requirement: Comparison rows for repos and members

The user MUST be able to fetch comparison rows for all repos or all members in one request each: the current period, the previous period of the same length, and 12 monthly summaries.

#### Scenario: Repo overview

- **WHEN** the client requests `GET /api/v1/overview/repos?from=2026-07&to=2026-10`
- **THEN** the answer names the previous period (2026-04 to 2026-07) and has one row per tracked, enabled repo with `current`, `previous` and 12 `monthly` summaries ending at 2026-10

#### Scenario: Member overview

- **WHEN** the client requests `GET /api/v1/overview/members?from=2026-09`
- **THEN** each row is a member active in the current or previous period across all repos, or an active account no member claims (`member_id` null), and excluded accounts are absent

#### Scenario: Missing data is null

- **WHEN** a row has no data for a metric in a period
- **THEN** that summary field is `null`, not 0
