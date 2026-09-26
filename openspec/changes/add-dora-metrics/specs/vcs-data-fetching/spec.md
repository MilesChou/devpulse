# Spec Delta

## ADDED Requirements

### Requirement: Capture deployment and remediation facts for every PR

The PR sync SHALL record, for every PR: title, labels, base branch, head branch and merge commit SHA. For a PR whose body references the PR it reverts in GitHub's `Reverts owner/repo#N` form, it SHALL also record that number. For a merged PR, it SHALL additionally record the earliest author time among the PR's commits.

#### Scenario: Merged PR carries its earliest commit time

- **WHEN** a merged PR has commits authored at `09:00` and `11:00`
- **THEN** the stored PR carries `09:00` as its earliest commit time

#### Scenario: Revert target is recorded

- **WHEN** a PR's body contains `Reverts MilesChou/devpulse#42` and the PR belongs to `MilesChou/devpulse`
- **THEN** the stored PR records that it reverts PR #42

#### Scenario: Cross-repo revert reference is ignored

- **WHEN** a PR's body contains `Reverts other/repo#42`
- **THEN** no reverted PR number is recorded

### Requirement: Open PRs are refreshed on every sync

Every repo sync SHALL re-fetch each PR that is stored as open, so that a later merge or close is recorded. A PR that can no longer be fetched SHALL NOT abort the sync.

#### Scenario: PR merged after the first sync

- **WHEN** PR #7 was open at the first sync and has since been merged
- **THEN** after the next sync PR #7 is stored as merged with its merge time

### Requirement: Fetch incident issues

Every repo sync SHALL fetch every issue (open or closed, excluding pull requests) that carries the repo's incident label, and store its number, title, creation time and closing time. The stored set SHALL mirror the upstream set on each sync, so that an issue that no longer carries the label is removed. A failure to fetch incidents SHALL be reported without aborting the rest of the repo sync.

#### Scenario: Incident issues are mirrored

- **WHEN** the repo has two issues labelled `incident`, one open and one closed
- **THEN** both are stored, and only the closed one has a closing time

#### Scenario: Pull requests with the label are excluded

- **WHEN** a pull request carries the `incident` label
- **THEN** it is not stored as an incident
