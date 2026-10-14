## ADDED Requirements

### Requirement: Comparison rows use the median lead time

The lead time in an overview summary MUST be the median, named `lead_time_p50_hours`, so rows compare what a typical PR takes rather than a mean dominated by a few long ones.

#### Scenario: Summary field

- **WHEN** a repo's merged PRs in the period take 1 h, 3 h and 100 h
- **THEN** its summary has `lead_time_p50_hours` 3

### Requirement: Reports carry a build duration summary

Every metrics report MUST carry `build_duration` (count, mean, median and p90 in seconds) and each day of `daily_build_duration` its median (`p50_seconds`), and overview summaries MUST report the median build duration as `build_p50_seconds`.

#### Scenario: Report

- **WHEN** a window has builds of 30 s, 60 s, 90 s and 120 s
- **THEN** `build_duration` is count 4 with `p50_seconds` 75

