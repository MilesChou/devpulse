# CI Data Fetching

Fetches build history from CI services. Internally a provider abstraction isolates concrete CI services (Travis CI is the first implementation), including translation of build attributes (`is_post_merge` / `is_pull_request` / `is_deploy_event`).

## Requirements

### Requirement: Multiple CI services share one command surface

The user MUST be able to query repositories backed by different CI services (e.g. some on Travis, some on GitHub Actions) with the same commands and arguments, without having to remember which command applies to which repository.

#### Scenario: Different CI service, same command

- **WHEN** the user queries CI failure rate for repo A (Travis) and repo B (GitHub Actions) for the same month
- **THEN** both queries use the same command and return results in the same shape

### Requirement: First version ships with at least one provider, and that provider is Travis CI

The user MUST be able to query Travis CI data in the first version, because Travis is the environment validated by the original prototype. The system MUST implement support for at least one concrete CI service.

#### Scenario: Travis is available

- **WHEN** the user configures a Travis API token and points the tool at a Travis-hosted repo
- **THEN** the system can fetch build records and compute metrics

### Requirement: Adding a new CI provider must not force existing repos to be reconfigured

The user SHOULD NOT have to migrate existing configuration when the system later adds support for additional CI services (e.g. GitHub Actions, CircleCI). Repos that already work continue to work with their existing configuration.

#### Scenario: Adding a new CI provider leaves existing repos untouched

- **WHEN** the system later adds GitHub Actions support
- **THEN** repos previously configured for Travis continue to work without any configuration change

### Requirement: Fetch CI runs for a given month

The user MUST be able to specify a repository and a month and see every CI run in that month, regardless of outcome (succeeded, failed, errored, cancelled).

#### Scenario: Fetch CI runs for a single month

- **WHEN** the user specifies a repo and a month
- **THEN** the system returns every CI run for that month, each with its status (succeeded / failed / errored / cancelled), start time, duration, and associated commit

### Requirement: Distinguish the nature of each CI run

The user MUST be able to tell whether a CI run was triggered by a PR, by a post-merge push to the main branch, or by a deploy pipeline — so that failures unrelated to individual ownership can be excluded.

#### Scenario: PR-stage run is identifiable

- **WHEN** the user inspects a CI run
- **THEN** the system reports whether it was triggered by a PR, whether it ran after merge, and whether it is part of a deploy pipeline

#### Scenario: Failure rate defaults to PR-stage runs only

- **WHEN** the user computes failure rate without further options
- **THEN** the system excludes post-merge and deploy runs by default, since those do not reflect individual PR quality

### Requirement: Fetch the full log of a failed run

The user MUST be able to retrieve the complete log of a failed CI run so the failure can be diagnosed.

#### Scenario: Retrieve failure log

- **WHEN** the user requests the details of a failed CI run
- **THEN** the system returns the full log text for that run

### Requirement: Closed months must not be re-fetched from the external service

The user MUST get an immediate response when re-querying a month that has already ended, rather than triggering another round trip to the CI service.

#### Scenario: Past months are served from cache

- **WHEN** the user queries a month that has ended (e.g. it is currently 2026-05 and the user queries 2026-04) and that month has been fetched before
- **THEN** the system serves the response from local data without calling the CI service

#### Scenario: The current month can still be re-fetched

- **WHEN** the user queries a month that is still in progress (e.g. it is currently 2026-05 and the user queries 2026-05)
- **THEN** the system allows re-fetching so new runs are reflected
