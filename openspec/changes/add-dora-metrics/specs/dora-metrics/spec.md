# Spec Delta

## Purpose

Reports the four DORA delivery-performance metrics per repo and time window, using merges into the default branch as deployments and revert/hotfix PRs plus incident issues as failures.

## ADDED Requirements

### Requirement: A deployment is a PR merged into the default branch

The system SHALL treat every PR merged into the repo's default branch as exactly one deployment, timestamped at its merge time. PRs merged into any other branch SHALL NOT count as deployments.

#### Scenario: Merge into the default branch counts

- **WHEN** a PR with base branch `main` is merged at `2026-05-10T12:00Z` and the repo's default branch is `main`
- **THEN** one deployment at `2026-05-10T12:00Z` is counted for May 2026

#### Scenario: Merge into another branch does not count

- **WHEN** a PR with base branch `release/1.x` is merged and the repo's default branch is `main`
- **THEN** it is not counted as a deployment

#### Scenario: Unknown default branch

- **WHEN** the repo's default branch has never been fetched
- **THEN** the DORA section states that the default branch is unknown and tells the user to run `devpulse repo refresh`, instead of reporting numbers

### Requirement: Deployment Frequency

The system SHALL report, for a window, the number of deployments, the average deployments per week (deployments ÷ window days × 7), and the number of distinct UTC calendar days with at least one deployment.

#### Scenario: Frequency over one month

- **WHEN** May 2026 (31 days) contains 10 deployments on 6 distinct days
- **THEN** the report shows 10 deployments, 2.26 per week, and 6 deploy days

### Requirement: Lead Time for Changes

The system SHALL report, over deployments in the window, the average, p50 and p90 hours from the earliest commit author time in the PR to the merge time. Deployments without a known earliest commit time SHALL be excluded and SHALL NOT count as zero. A negative duration SHALL be reported as zero.

#### Scenario: Lead time from first commit

- **WHEN** a deployment's earliest commit was authored at `09:00` and it merged at `15:00` the same day
- **THEN** its lead time is 6 hours

#### Scenario: Missing first-commit time is excluded

- **WHEN** a deployment has no recorded earliest commit time
- **THEN** it is excluded from the lead-time sample and the sample count reflects that

### Requirement: Change Failure Rate

The system SHALL report the ratio of remediation deployments to all deployments in the window. A deployment is a remediation when its title starts with the word "revert" (case-insensitive), when it carries the repo's hotfix label, or when its head branch starts with `hotfix/`. Each deployment SHALL count at most once. The report SHALL show the revert and hotfix counts separately.

#### Scenario: Reverts and hotfixes both count

- **WHEN** a window has 20 deployments, of which 1 is titled `Revert "feat: x"` and 1 carries the `hotfix` label
- **THEN** the change failure rate is 10.0% with reverts=1 and hotfixes=1

#### Scenario: No deployments

- **WHEN** a window has no deployments
- **THEN** the change failure rate is reported as not applicable rather than 0%

### Requirement: Failed Deployment Recovery Time

The system SHALL report the average, p50 and p90 hours to recover, drawn from two sources and attributed to the window by the recovery time:
(a) a revert deployment whose reverted PR is known and merged, measured from the reverted PR's merge to the revert's merge;
(b) an incident issue closed in the window, measured from its creation to its closing.
The report SHALL show the sample count of each source.

#### Scenario: Recovery via revert

- **WHEN** PR #10 merged at `10:00` and PR #11, whose body says `Reverts owner/repo#10`, merged at `12:30`
- **THEN** a recovery sample of 2.5 hours from source "revert" is recorded

#### Scenario: Recovery via incident issue

- **WHEN** an issue labelled `incident` was opened at `08:00` and closed at `11:00` inside the window
- **THEN** a recovery sample of 3 hours from source "incident" is recorded

#### Scenario: Open incident is not a sample

- **WHEN** an incident issue is still open at report time
- **THEN** it contributes no recovery sample

### Requirement: DORA metrics appear in the metrics command

`devpulse metrics <owner/name>` SHALL print a DORA section that contains all four metrics for the requested window, next to the existing metrics.

#### Scenario: DORA section is printed

- **WHEN** the user runs `devpulse metrics MilesChou/devpulse --from 2026-05`
- **THEN** the output contains Deployment Frequency, Lead Time for Changes, Change Failure Rate and Failed Deployment Recovery Time lines for May 2026
