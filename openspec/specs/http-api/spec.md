# HTTP API

## Purpose

`devpulse serve` exposes the tracked repos and their metrics as a read-only JSON API, so clients other than the CLI (the desktop dashboard first) can show them without direct database access or any GitHub / CI credential.

## Requirements

### Requirement: Metrics over HTTP match the CLI

The user MUST get the same numbers from the API as from `devpulse metrics` for the same repo and month window, so switching between the terminal and a dashboard never shows two different answers.

#### Scenario: Same window, same numbers

- **WHEN** the user runs `devpulse metrics owner/name --from 2026-05` and requests `GET /api/v1/repos/owner/name/metrics?from=2026-05`
- **THEN** both report the same failure rate, builds per PR, PR lead time, review wait, size distribution, daily build duration, and DORA metrics

#### Scenario: Default window

- **WHEN** the user omits `from` and `to`
- **THEN** the window is the current UTC month, as in the CLI

### Requirement: Absent DORA data is explicit

The user MUST be able to tell "cannot be computed" apart from "zero" in the DORA section of a report.

#### Scenario: Default branch unknown

- **WHEN** the repo's default branch has not been fetched yet
- **THEN** the report's `dora` field is `null`, because deployments are defined as merges into the default branch

#### Scenario: No deployments

- **WHEN** the window has no deployments
- **THEN** `dora.change_failure_rate` is `null` rather than 0

### Requirement: Month-by-month trend in one request

The user MUST be able to fetch one report per month for a range of months in a single request, so trend charts and month-over-month comparisons do not need one round trip per month.

#### Scenario: Twelve-month trend

- **WHEN** the user requests `GET /api/v1/repos/owner/name/metrics/monthly?from=2025-10&to=2026-10`
- **THEN** the response contains twelve reports, oldest first, each covering exactly one month

#### Scenario: Oversized range is rejected

- **WHEN** the requested range spans more than 36 months
- **THEN** the system answers 400 with an error message instead of running an unbounded number of queries

### Requirement: The API is authenticated unless it is only reachable locally

The user MUST NOT be able to expose the API to other hosts without a token, because it reveals the data of every tracked repo.

#### Scenario: Token required on /api/

- **WHEN** a request to any `/api/` path lacks `Authorization: Bearer <DEVPULSE_API_TOKEN>` or carries a wrong token
- **THEN** the system answers 401 and returns no data, including for paths that do not exist

#### Scenario: Public bind without a token is refused

- **WHEN** the user starts `devpulse serve` with a non-loopback `HTTP_ADDR` (e.g. `0.0.0.0:8080`) and no `DEVPULSE_API_TOKEN`
- **THEN** the command exits with an error naming the missing variable, before opening the database

#### Scenario: Liveness probe without a token

- **WHEN** a load balancer or the dashboard requests `GET /healthz`
- **THEN** the system answers 200 without requiring the token

### Requirement: Errors are machine-readable and do not leak internals

The user MUST receive JSON errors whose status tells a client what to fix, and MUST NOT see database or upstream error details in responses.

#### Scenario: Error classes

- **WHEN** the request has a malformed month, an unknown repo, or fails inside the server
- **THEN** the system answers 400, 404, or 500 respectively, with a body `{"error": "..."}`; for 500 the details go to the server log only

### Requirement: The response shape is a tested contract

The user MUST be able to rely on the JSON field names and ordering staying stable unless deliberately changed, because separately built clients decode them.

#### Scenario: Contract drift fails tests

- **WHEN** a change alters the metrics JSON shape
- **THEN** the Go golden-file tests under `internal/http/testdata/` and the desktop client's decoding tests of the same files fail until both sides are updated

#### Scenario: Stable size-bucket axis

- **WHEN** a month has PRs in only some size buckets
- **THEN** the size distribution still lists XS, S, M, L, XL in that order (zero counts included), adding `unknown` only when some PRs lack a bucket
