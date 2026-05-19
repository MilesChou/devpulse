# Metrics Persistence

Persists fetched raw data and aggregated results so they can be queried later and accumulated long-term, treating closed months as immutable historical fact.

## Requirements

### Requirement: Fetched data is retained and not re-fetched

The user MUST get an immediate response when querying data the system has already fetched for a given (repo, month), without triggering another call to the external service.

#### Scenario: Second query is served instantly

- **WHEN** the user re-queries the metrics for the same repo and month
- **THEN** the system responds without calling the external API

### Requirement: Cross-month trend queries

The user MUST be able to query metrics for the same repo or member across multiple months and see long-term trends (e.g. failure rate over 6 months).

#### Scenario: Six-month trend

- **WHEN** the user queries failure rate for a repo from 2026-01 to 2026-06
- **THEN** the system returns 6 monthly rows, each containing that month's failure rate

### Requirement: Closed months are immutable

The user MUST NOT have data for closed months overwritten by re-running the tool, because past facts must not change when re-fetched.

#### Scenario: Past months do not change

- **WHEN** the user re-runs the tool today (2026-05) against 2026-04 data
- **THEN** the system uses the existing 2026-04 data and does not re-fetch from the external service

#### Scenario: The current month can still be updated

- **WHEN** the user runs the tool today (2026-05) against 2026-05 data
- **THEN** the system allows updates, because the month is still in progress

### Requirement: System upgrades must not destroy historical data

The user MUST NOT have to re-fetch past data from the external service when the system later adds new analysis fields (e.g. a "commit message" field), because the original responses have been retained and can be re-parsed.

#### Scenario: Backfill new fields without re-calling the API

- **WHEN** the system later adds a field sourced from the external response
- **THEN** the system can parse that field out of locally retained raw responses and backfill historical records without calling the external service
