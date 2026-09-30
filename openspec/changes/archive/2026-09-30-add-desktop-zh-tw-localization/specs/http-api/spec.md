## MODIFIED Requirements

### Requirement: Month-by-month trend in one request

The user MUST be able to fetch one report per month for a range of months in a single request, so trend charts and month-over-month comparisons do not need one round trip per month.

#### Scenario: Twelve-month trend

- **WHEN** the user requests `GET /api/v1/repos/owner/name/metrics/monthly?from=2025-10&to=2026-10`
- **THEN** the response contains twelve reports, oldest first, each covering exactly one month

#### Scenario: Oversized range is rejected

- **WHEN** the requested range spans more than 120 months
- **THEN** the system answers 400 with an error message instead of running an unbounded number of queries
