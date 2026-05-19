# VCS Data Fetching

Fetches PR, commit-author, and PR-review history from GitHub. Handles rate limiting, bulk fetching, and bot filtering.

## Requirements

### Requirement: List PRs for a given month

The user MUST be able to specify a repo and a month, and have the system list every PR created in that month — including merged, closed, rejected, and still-draft PRs.

#### Scenario: List PRs for a single month

- **WHEN** the user specifies `repo="owner/name"` and `month="2026-04"`
- **THEN** the system returns every PR created in that month

### Requirement: Provide each PR's key timestamps and size data

The user MUST be able to retrieve, for every PR, the data needed to compute review latency and size bucketing: author, ready-for-review timestamp, first-review timestamp, merge status, and lines changed.

#### Scenario: Retrieve PR details

- **WHEN** the user requests details for a PR
- **THEN** the system returns author, ready-for-review timestamp, first-review timestamp (if any), lines changed, and current status (open / closed / merged)

### Requirement: Map commits back to contributors

The user MUST be able to take a commit referenced by a CI build and look up the commit's author, so builds can be attributed to team members.

#### Scenario: Commit maps to a GitHub account

- **WHEN** the user provides a set of commits
- **THEN** the system returns the GitHub account for each commit (commits with no matching account may be omitted)

### Requirement: Exclude automation-bot activity

The user MUST be able to configure a list of bot accounts to exclude, so PRs opened by bots and reviews left by bots (e.g. dependabot, Copilot auto-review) do not pollute statistics.

#### Scenario: Dependabot PRs are excluded

- **WHEN** a PR's author is in the user's bot list
- **THEN** that PR does not appear in downstream PR review-latency or size-bucket statistics

### Requirement: Repeated queries must not keep hitting the external service

The user MUST be able to re-run the same (repo, month) query without forcing a round trip to the external API every time, to avoid latency and quota waste.

#### Scenario: Second query is served from cache

- **WHEN** the user re-runs a query for the same repo and month
- **THEN** the system reuses the previous result and does not re-call the external service

### Requirement: A transient external outage must not abort the whole batch

The user MUST see the system back off and retry when the external service is temporarily busy (rate limit, 503, etc.), rather than the entire batch fetch failing outright.

#### Scenario: Rate-limited mid-batch still completes

- **WHEN** the external service responds with rate-limit signals partway through a fetch
- **THEN** the system waits and retries, eventually completing the batch (or surfacing a clear error after repeated retries fail)
