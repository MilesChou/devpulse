## ADDED Requirements

### Requirement: Compare repos for a period

The user MUST be able to see every tracked, enabled repo side by side for a period: PRs opened and merged, PR lead time, builds per PR, CI failure rate, average build duration, review wait, and deployments per week.

#### Scenario: Repo table

- **WHEN** the user opens the Overview with a period selected
- **THEN** there is one row per tracked, enabled repo with those metrics for the period, and "—" where a repo has no data for a metric

#### Scenario: Finding the repo that got slower

- **WHEN** a repo's average build duration rose from 120 s in the previous period to 180 s in the current one
- **THEN** its row shows 180 s with a +50 % change marked as worse

### Requirement: Compare members for a period

The user MUST be able to see members side by side across all repos: PRs opened and merged, PR lead time, builds per PR, CI failure rate, average build duration, and review wait.

#### Scenario: Member table

- **WHEN** the user opens the Overview
- **THEN** there is one row per member with activity in any repo in the current or previous period, then one row per active account no member claims, and excluded accounts (bots) never appear

#### Scenario: Unmapped account

- **WHEN** a row is an account no member claims
- **THEN** it is marked as unmapped and offers Map…, which opens the People page with a new member prefilled with that account

### Requirement: Change against the previous period

Every metric in a comparison row MUST show its change against the previous period of the same length, and the change MUST be marked when it goes in the worse direction by more than a noise floor.

#### Scenario: Previous period

- **WHEN** the period is 2026-07 to 2026-09 (three months)
- **THEN** the previous period is 2026-04 to 2026-06

#### Scenario: Worse direction

- **WHEN** CI failure rate rises by more than 2 percentage points, or lead time, builds per PR, build duration or review wait rise by more than 10 %, or deployments per week fall by more than 10 %
- **THEN** the change is shown in the warning colour; smaller or better changes are shown in a neutral colour

#### Scenario: No data before

- **WHEN** the previous period has no data for a metric
- **THEN** no change is shown for it

### Requirement: Sort and see the trend

The user MUST be able to sort each table by any metric and see a 12-month trend for each row.

#### Scenario: Sort

- **WHEN** the user clicks a column header
- **THEN** the table sorts by that column, worst first; clicking again reverses the order; rows without data for that column go last

#### Scenario: Sparkline

- **WHEN** a table is sorted by a metric
- **THEN** each row shows a sparkline of that metric for the 12 months ending at the period's end, with gaps for months without data

### Requirement: Drill down from a row

The user MUST be able to open the dashboard for a row.

#### Scenario: Repo row

- **WHEN** the user clicks a repo row
- **THEN** the Dashboard opens for that repo and the same period, showing everyone

#### Scenario: Member row

- **WHEN** the user clicks a member row
- **THEN** the Dashboard opens for All repos and that member, for the same period

#### Scenario: Unmapped account row

- **WHEN** the user clicks the name of an account no member claims
- **THEN** the Dashboard opens for All repos and that account alone, so a person can be looked at before anyone maps them
