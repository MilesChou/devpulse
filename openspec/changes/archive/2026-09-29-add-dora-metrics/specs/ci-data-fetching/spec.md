# Spec Delta

## ADDED Requirements

### Requirement: PR-triggered builds are linked to their PR

Every repo sync SHALL link each PR-triggered build that carries no PR number to the stored PR whose head branch equals the build's branch and that was open when the build started. A build that matches more than one PR SHALL stay unlinked. A PR number reported by the CI provider SHALL NOT be overwritten, and push builds SHALL NOT be linked.

#### Scenario: Build on a PR branch is linked

- **WHEN** PR #5 from branch `feature/login` was opened at `09:00` and is still open, and a PR-triggered build on `feature/login` started at `10:00`
- **THEN** the build is stored as belonging to PR #5

#### Scenario: Reused branch name resolves by time

- **WHEN** PR #1 from `feat/a` was open from `10:00` to `12:00`, PR #2 from `feat/a` opened at `13:00`, and a PR-triggered build on `feat/a` started at `14:00`
- **THEN** the build is stored as belonging to PR #2

#### Scenario: Ambiguous build stays unlinked

- **WHEN** two PRs from branches named `main` are open at the same time and a PR-triggered build on `main` started while both were open
- **THEN** the build is stored without a PR number
