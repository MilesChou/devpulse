# Tool Configuration

Defines the member list, repo list, bot exclusion list, PR size buckets, and human-signal rules. Static values live in config; values that change live in the DB and are maintained via the CLI. The example configuration MUST be decoupled from any specific organisation.

## Requirements

### Requirement: Maintain a team-member list

The user MUST be able to maintain a member list where each member has a display name and a corresponding GitHub account, so reports can attribute commits / PRs to a "human name" instead of a GitHub login.

#### Scenario: Reports show display name rather than login

- **WHEN** the user runs a monthly report
- **THEN** the "member" column shows the configured display name (e.g. "Member1") rather than the GitHub login (e.g. "user-1")

### Requirement: Multiple groups for different teams or scenarios

The user MUST be able to define multiple groups, each with its own set of repos and members, so "my team", "neighbouring team", and "any future deployment scenario" can be observed independently.

#### Scenario: Switch group to view a different team

- **WHEN** the user runs a monthly report and specifies a different group
- **THEN** the system reports statistics from that group's repos and members only, without mixing in other groups

### Requirement: The tool is decoupled from any specific organisation

The user MUST, on first acquiring this tool, see example configuration that contains no real organisation data (no real org names, no real member names), so the tool can be redeployed across different teams and sensitive data is not leaked.

#### Scenario: Initial template is neutral

- **WHEN** the user first obtains the tool and copies the example configuration
- **THEN** the example contains only placeholders (e.g. `your-org/your-repo`, `Member1`), with no real organisation or member names

### Requirement: Configurable automation-bot exclusion list

The user MUST be able to configure which bot accounts to exclude (e.g. dependabot, Copilot auto-review). The default list MUST already include common bots.

#### Scenario: Common bots excluded by default

- **WHEN** the user runs a monthly report on a fresh install
- **THEN** dependabot, Copilot auto-review, and similar bots do not pollute statistics by default

#### Scenario: User can add a new bot

- **WHEN** the user adds a new bot to the exclusion list
- **THEN** subsequent reports stop counting that bot's activity

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

The user MUST provide external API credentials (GitHub token, Travis token) via environment variables rather than hard-coding them in configuration files, to avoid accidental commits.

#### Scenario: Missing token surfaces a clear error

- **WHEN** the user runs a command without setting a required token
- **THEN** the system prints a clear error message identifying which token is missing, rather than failing with an opaque error after attempting the API call
