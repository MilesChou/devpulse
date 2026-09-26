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

### Requirement: Fetch incident issues

Every repo sync SHALL fetch every issue (open or closed, excluding pull requests) that carries the repo's incident label, and store its number, title, creation time and closing time. The stored set SHALL mirror the upstream set on each sync, so that an issue that no longer carries the label is removed. A failure to fetch incidents SHALL be reported without aborting the rest of the repo sync.

#### Scenario: Incident issues are mirrored

- **WHEN** the repo has two issues labelled `incident`, one open and one closed
- **THEN** both are stored, and only the closed one has a closing time

#### Scenario: Pull requests with the label are excluded

- **WHEN** a pull request carries the `incident` label
- **THEN** it is not stored as an incident
