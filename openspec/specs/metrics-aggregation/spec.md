# Metrics Aggregation

Aggregates raw build / PR / review data into engineering-efficiency metrics. This spec aligns with the four CI metrics listed in [CLAUDE.md](../../../CLAUDE.md): build failure rate, PR re-push count, PR merged-minus-created duration, and PR size distribution.

## Requirements

### Requirement: Monthly per-member-per-repo CI failure rate

The user MUST be able to see, for any given month, the CI failure rate of every (member, repo) pair as an individual observability metric.

#### Scenario: Monthly failure rate

- **WHEN** the user specifies a group (containing members and repos) and a month
- **THEN** the system returns, for every (member, repo) pair, the total run count, failure count, and failure rate

### Requirement: Failure rate defaults to runs the individual is responsible for

The user MUST, with no extra configuration, see post-merge and deploy runs automatically excluded from failure-rate calculations, so those failures are not mistakenly attributed to individuals.

#### Scenario: Post-merge runs excluded by default

- **WHEN** the user does not explicitly opt to include post-merge runs
- **THEN** runs that happened after merging to the main branch are not counted in the individual failure rate

#### Scenario: Post-merge runs can be opted back in

- **WHEN** the user explicitly opts to include post-merge runs
- **THEN** those runs are included

### Requirement: Compute the wait from PR ready to first review

The user MUST be able to see, for every PR, the number of hours between "marked ready for review" and "first review received", as a metric for team review responsiveness.

#### Scenario: PR has been reviewed

- **WHEN** a PR has a ready-for-review timestamp and has received at least one review
- **THEN** the system reports the difference between the two timestamps in hours

#### Scenario: PR has not been reviewed by month end

- **WHEN** a PR was marked ready but had received no review by month end
- **THEN** the system uses month end (or "now", whichever is earlier) as a lower bound on the wait time

#### Scenario: Still-draft PR is not counted

- **WHEN** a PR is still a draft (never marked ready)
- **THEN** that PR is excluded from review-latency statistics

### Requirement: Bucket PRs by size

The user MUST be able to see every PR placed into a size bucket (e.g. XS / S / M / L / XL) based on lines changed, so review latency can be sliced by size (small PRs should be reviewed faster).

#### Scenario: Small PR is bucketed as XS

- **WHEN** a PR changes 49 lines and the size config is `XS<50, S<200, …`
- **THEN** that PR is bucketed as XS

#### Scenario: Size config is user-tunable

- **WHEN** the user adjusts size-bucket boundaries
- **THEN** subsequent classification uses the new boundaries

### Requirement: Compute PR lead time from created to merged

The user MUST be able to see, for every merged PR, the number of hours between `created_at` and `merged_at` as a team-level PR lead-time metric (ideal value = 24h).

#### Scenario: Lead time for a merged PR

- **WHEN** a PR has been merged
- **THEN** the system reports the difference between `created_at` and `merged_at` in hours

#### Scenario: Unmerged PRs are excluded

- **WHEN** a PR is open / closed-without-merge / draft
- **THEN** that PR is not counted in lead-time statistics

#### Scenario: Monthly slice

- **WHEN** the user specifies a month
- **THEN** the system returns the distribution of lead times for PRs merged in that month (including at least one of mean, p50, p90, etc.)

### Requirement: Month-over-month comparison

The user MUST, when viewing the current month's metrics, also see the direction (up / down / flat) and delta versus the previous month, so trends are easy to read.

#### Scenario: Failure-rate increase is visible

- **WHEN** the current month's failure rate is 5% and the previous month's was 3%
- **THEN** the result is annotated "↑ +2%"

### Requirement: Daily build-duration trend

The user MUST be able to see, per repo, the duration of each successful build day-by-day, in order to spot CI slowdown trends.

#### Scenario: Daily build duration

- **WHEN** the user queries daily build duration for a repo and month
- **THEN** the system returns per-day duration statistics (e.g. median, max, or the full sample set)

### Requirement: Compute build re-push count per PR

The user MUST be able to see "how many builds, on average, each PR triggered" as a proxy for "shipped in one push" (ideal value = 1).

#### Scenario: A PR triggers multiple builds

- **WHEN** a single PR triggered 3 builds
- **THEN** the system records the PR's build count as 3, available for computing monthly averages
