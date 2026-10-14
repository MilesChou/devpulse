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

### Requirement: Build duration summary

The system SHALL report the count, mean, median and p90 of the durations of the builds started in a window, over the builds themselves rather than over daily values, alongside a daily series that carries each day's mean and median.

#### Scenario: Over builds, not days

- **WHEN** a window has builds of 30 s, 60 s, 90 s and 120 s spread over two days
- **THEN** its median build duration is 75 s over 4 builds
