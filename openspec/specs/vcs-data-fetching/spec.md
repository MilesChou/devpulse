# VCS Data Fetching

## Purpose

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

#### Scenario: Bot reviews do not count as the first review

- **WHEN** a bot (e.g. Copilot) reviews a PR a minute after it becomes ready and a person reviews it two hours later
- **THEN** the PR's review wait is two hours

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

### Requirement: Capture deployment and remediation facts for every PR

The PR sync SHALL record, for every PR: title, labels, base branch, head branch and merge commit SHA. For a PR whose body references the PR it reverts in GitHub's `Reverts owner/repo#N` form, it SHALL also record that number. For a merged PR, it SHALL additionally record the earliest author time among the PR's commits.

#### Scenario: Merged PR carries its earliest commit time

- **WHEN** a merged PR has commits authored at `09:00` and `11:00`
- **THEN** the stored PR carries `09:00` as its earliest commit time

#### Scenario: Implausible commit times are bounded

- **WHEN** a merged PR has a commit whose author time is before 1971, or later than the PR's merge time
- **THEN** a pre-1971 time is ignored, and a time later than the merge is stored as the merge time

#### Scenario: Earliest commit time fetch failure does not fail the PR

- **WHEN** fetching a merged PR's commits fails
- **THEN** the PR is stored without an earliest commit time, and the sync continues with the next PR

#### Scenario: Revert target is recorded

- **WHEN** a PR's body contains `Reverts MilesChou/devpulse#42` and the PR belongs to `MilesChou/devpulse`
- **THEN** the stored PR records that it reverts PR #42

#### Scenario: Cross-repo revert reference is ignored

- **WHEN** a PR's body contains `Reverts other/repo#42`
- **THEN** no reverted PR number is recorded

### Requirement: Open PRs and PRs updated upstream are refreshed on every sync

Every repo sync SHALL re-fetch each stored PR that is stored as open or that the upstream reports as updated at or after the repo's PR watermark, so that a later merge, close or reopen is recorded. The watermark SHALL be the upstream update time of the most recently updated PR in the listing that was used, and SHALL NOT advance when any listed PR failed to refresh. A re-fetched PR whose upstream update time is older than the listing reported SHALL NOT be written and SHALL count as a failed refresh. A PR that can no longer be fetched SHALL NOT abort the sync.

#### Scenario: PR merged after the first sync

- **WHEN** PR #7 was open at the first sync and has since been merged
- **THEN** after the next sync PR #7 is stored as merged with its merge time

#### Scenario: Closed PR reopened and merged

- **WHEN** PR #2 was stored as closed, and upstream it was later reopened and merged
- **THEN** after the next sync PR #2 is stored as merged with its merge time

#### Scenario: Unchanged closed PR is not re-fetched

- **WHEN** PR #3 is stored as merged and has not been updated upstream since the last sync
- **THEN** the next sync does not fetch PR #3

#### Scenario: Stale copy is rejected

- **WHEN** PR #2 is listed as updated at `10:00`, but the fetched PR reports an update time of `09:00`
- **THEN** PR #2 is not written, and the watermark is not advanced

#### Scenario: Failed refresh is retried

- **WHEN** a listed PR fails to refresh
- **THEN** the watermark is not advanced, and the next sync lists that PR again

### Requirement: Stored PRs missing DORA facts are completed

Every repo sync SHALL re-fetch each stored PR, at or above the repo's PR sync floor, that has no recorded base branch or that is merged without an earliest commit time, so that a store synced before these facts existed fills in its history. The pass SHALL proceed in ascending PR number, SHALL stop at the first failure, and SHALL resume from stored state on the next sync. A failure SHALL be reported without aborting the rest of the repo sync.

#### Scenario: Pre-DORA rows are completed

- **WHEN** merged PR #3 was stored before base branches were recorded
- **THEN** after the next sync PR #3 carries its base branch and earliest commit time, and counts as a deployment

#### Scenario: Completion resumes after a failure

- **WHEN** completing PR #3 fails during a sync
- **THEN** the pass stops at #3, the CI build sync still runs, and the next sync resumes at #3

### Requirement: Fetch incident issues

Every repo sync SHALL fetch every issue (open or closed, excluding pull requests) that carries the repo's incident label, and store its number, title, creation time and closing time. The stored set SHALL mirror the upstream set on each sync, so that an issue that no longer carries the label is removed. A failure to fetch incidents SHALL be reported without aborting the rest of the repo sync.

#### Scenario: Incident issues are mirrored

- **WHEN** the repo has two issues labelled `incident`, one open and one closed
- **THEN** both are stored, and only the closed one has a closing time

#### Scenario: Pull requests with the label are excluded

- **WHEN** a pull request carries the `incident` label
- **THEN** it is not stored as an incident

