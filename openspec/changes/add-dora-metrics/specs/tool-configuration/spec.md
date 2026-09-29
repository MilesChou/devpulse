# Spec Delta

## ADDED Requirements

### Requirement: Incident and hotfix labels are configurable per repo

The user MUST be able to set, per repo, the label that marks incident issues (`incident-label`, default `incident`) and the label that marks hotfix PRs (`hotfix-label`, default `hotfix`) through `devpulse repo config`. Label matching SHALL be case-insensitive. A changed hotfix label SHALL affect the next metrics run without a re-sync. A changed incident label SHALL take effect on the next sync.

#### Scenario: Defaults

- **WHEN** the user has not configured either label
- **THEN** `devpulse repo config get <repo>` shows `incident-label=incident` and `hotfix-label=hotfix`

#### Scenario: Custom hotfix label

- **WHEN** the user runs `devpulse repo config set <repo> hotfix-label urgent-fix`
- **THEN** merged PRs labelled `Urgent-Fix` count as hotfix remediations in the next metrics run

#### Scenario: Empty label is rejected

- **WHEN** the user sets either label to an empty or whitespace-only value
- **THEN** the command fails with a validation error and the stored value is unchanged
