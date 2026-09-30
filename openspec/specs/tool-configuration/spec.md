# Tool Configuration

## Purpose

Defines the member list, repo list, bot exclusion list, PR size buckets, and human-signal rules. Static values live in config; values that change live in the DB and are maintained via the CLI or the HTTP API (and so the desktop dashboard). The example configuration MUST be decoupled from any specific organisation.
## Requirements
### Requirement: Maintain a team-member list

The user MUST be able to maintain a member list where each member has a display name and one or more GitHub accounts, so reports can attribute commits / PRs to a "human name" instead of a GitHub login, and one person's accounts count together. An account belongs to at most one member. Accounts compare case-insensitively.

#### Scenario: Reports show display name rather than login

- **WHEN** the user views the per-member breakdown of a repo and month
- **THEN** the row shows the configured display name (e.g. "Member1") rather than the GitHub login (e.g. "user-1")

#### Scenario: One person, several accounts

- **WHEN** a member has the accounts `user-1` and `user-1-work`
- **THEN** that member's metrics include work by both accounts

#### Scenario: Unmapped accounts stay visible

- **WHEN** an account with activity in the window belongs to no member
- **THEN** the breakdown lists it under its login, so the user can map it

### Requirement: Multiple groups for different teams or scenarios

The user MUST be able to define multiple teams, each a set of members, so "my team" and "neighbouring team" can be observed independently. A member may be in several teams. Teams do not own repos: the repo is chosen separately, and a team view shows that team's work in the chosen repo.

#### Scenario: Switch team to view a different team

- **WHEN** the user views a repo's metrics for a different team
- **THEN** the system reports statistics from that team's members' work only, without mixing in other people's

### Requirement: The tool is decoupled from any specific organisation

The user MUST, on first acquiring this tool, see example configuration that contains no real organisation data (no real org names, no real member names), so the tool can be redeployed across different teams and sensitive data is not leaked.

#### Scenario: Initial template is neutral

- **WHEN** the user first obtains the tool and copies the example configuration
- **THEN** the example contains only placeholders (e.g. `your-org/your-repo`, `Member1`), with no real organisation or member names

### Requirement: Configurable automation-bot exclusion list

The user MUST be able to configure which bot accounts to exclude (e.g. dependabot, Copilot auto-review). The default list MUST already include common bots: `dependabot`, `github-actions`, and `copilot-pull-request-reviewer`. Excluded accounts' PRs, builds, and reviews count in no metric except DORA, where a merge into the default branch is a deployment whoever made it.

#### Scenario: Common bots excluded by default

- **WHEN** the user runs a monthly report on a fresh install
- **THEN** dependabot, Copilot auto-review, and similar bots do not pollute statistics by default

#### Scenario: User can add a new bot

- **WHEN** the user adds a new bot to the exclusion list
- **THEN** subsequent reports stop counting that bot's activity, without a re-sync

#### Scenario: One entry covers both API spellings

- **WHEN** the list contains `dependabot`
- **THEN** activity by `dependabot[bot]` (as GitHub's REST API names it) is excluded too

### Requirement: PR size buckets are tunable

The user MUST be able to adjust PR size-bucket boundaries (upper line-count limit for each of XS / S / M / L / XL), because the definition of "large PR" varies between teams.

#### Scenario: Use defaults

- **WHEN** the user has not adjusted boundaries
- **THEN** the system uses the built-in defaults

#### Scenario: Custom boundaries

- **WHEN** the user changes the XS upper limit to 100
- **THEN** subsequent classification uses 100 as the boundary

### Requirement: Failure-signal rules are configurable per repo

The user MUST be able to define per-repo rules of the form "this combination of log strings means a human error" (e.g. lint failure, test failure), so the system can auto-classify failure causes.

#### Scenario: Define a lint rule

- **WHEN** the user configures a repo with "if the log contains both `go vet` and `vet:` it is a lint failure"
- **THEN** any failed build on that repo whose log contains both strings is labelled as a lint-class failure

### Requirement: API credentials are provided via environment variables

The user MUST provide external API credentials (GitHub token, Travis token) and the DevPulse API token (`DEVPULSE_API_TOKEN`, for `devpulse serve`) to the Go service via environment variables rather than hard-coding them in configuration files, to avoid accidental commits. External credentials MUST stay on the host running the service: clients of the [HTTP API](../http-api/spec.md) never receive them, and the [desktop dashboard](../desktop-dashboard/spec.md) holds only the DevPulse API token, in the OS keychain.

#### Scenario: Missing token surfaces a clear error

- **WHEN** the user runs a command without setting a required token
- **THEN** the system prints a clear error message identifying which token is missing, rather than failing with an opaque error after attempting the API call

#### Scenario: Dashboard never sees GitHub or CI tokens

- **WHEN** the user connects the desktop dashboard to a DevPulse server
- **THEN** the dashboard asks only for the server URL and the DevPulse API token

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
