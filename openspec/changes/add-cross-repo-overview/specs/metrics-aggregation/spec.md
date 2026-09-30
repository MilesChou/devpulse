## ADDED Requirements

### Requirement: Metrics across a set of repos

The system SHALL compute every CI and PR metric over a set of repos by pooling the underlying PRs, builds and reviews, so averages and percentiles across repos are exact rather than averages of per-repo values.

#### Scenario: Pooled percentiles

- **WHEN** repo A has merged PRs with lead times 1 h and 3 h, and repo B one with 100 h
- **THEN** the lead time across A and B has avg 34.7 h and p50 3 h, not the average of the two repos' p50s

#### Scenario: Single repo unchanged

- **WHEN** the set holds one repo
- **THEN** every metric equals the per-repo metric for that repo

#### Scenario: DORA stays per repo

- **WHEN** metrics are computed across more than one repo
- **THEN** no DORA section is produced

### Requirement: Average build duration

The system SHALL report the average duration of the builds started in a window, weighted by build, alongside the daily build-duration series.

#### Scenario: Weighted average

- **WHEN** a window has 3 builds of 60 s on one day and 1 build of 300 s on another
- **THEN** the average build duration is 120 s, not the 180 s average of the two daily averages
