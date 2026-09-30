# Desktop Dashboard

## Purpose

A native desktop app (`desktop/`, Rust) that charts the metrics served by the [HTTP API](../http-api/spec.md) and manages which repos the server tracks. Data collection and every GitHub / CI credential stay with the Go service.

## Requirements

### Requirement: Connect with a server URL and an API token only

The user MUST be able to use the dashboard by entering the DevPulse server URL and its API token, without installing a database driver or handing the dashboard any GitHub or CI token.

#### Scenario: First launch

- **WHEN** the user starts the dashboard with no stored token
- **THEN** the settings panel opens and asks for the server URL and the API token

#### Scenario: Connection test

- **WHEN** the user presses Test
- **THEN** the dashboard reports whether the server is reachable and whether the token is accepted, as separate failure messages

### Requirement: The API token is kept in the OS keychain

The user MUST NOT find the API token in a plain-text settings file; it is stored in the operating system's credential store, per server URL.

#### Scenario: Token storage

- **WHEN** the user saves a token
- **THEN** it is written to the OS keychain (macOS Keychain, Windows Credential Manager, Secret Service), and the settings file holds only the server URL and the last selected repo

#### Scenario: Several servers

- **WHEN** the user switches to a different server URL without typing a token
- **THEN** the dashboard uses the token already stored for that URL, leaving other servers' tokens untouched

#### Scenario: Scripted launch

- **WHEN** the dashboard starts with `DEVPULSE_SERVER_URL` and `DEVPULSE_API_TOKEN` set
- **THEN** it uses them for that run only and writes neither to the settings file nor to the keychain

### Requirement: Show the project's CI metrics against their ideals

The user MUST see, for a selected repo and month window, the metrics listed in the project goals next to their ideal values.

#### Scenario: KPI cards

- **WHEN** the user selects a repo and a month
- **THEN** the dashboard shows CI failure rate (ideal 0%), builds per PR (ideal 1), PR lead time with p50 / p90 (ideal 24h), and review wait, plus the PR size distribution and the share of small (XS + S) PRs

#### Scenario: Empty month

- **WHEN** the window has no builds or PRs
- **THEN** the affected cards show "—" rather than a misleading 0

### Requirement: Show the DORA metrics

The user MUST see the four DORA metrics for the selected window, and MUST be told what is missing when the server cannot compute them.

#### Scenario: DORA cards

- **WHEN** the server reports a DORA section for the window
- **THEN** the dashboard shows deployment frequency, lead time for changes, change failure rate, and recovery time, each marked with its better direction, since the project goals set no DORA targets

#### Scenario: Default branch unknown

- **WHEN** the server reports no DORA section because the repo's default branch is unknown
- **THEN** the dashboard says to run `devpulse repo refresh <owner/name>` on the server instead of showing zeros

#### Scenario: No deployments

- **WHEN** the window has no deployments
- **THEN** change failure rate shows "—", not 0%

### Requirement: Month-over-month change and trends

The user MUST be able to see whether each metric improved against the previous month and how it moved over the last year.

#### Scenario: Change against the previous month

- **WHEN** a single month is selected and the previous month has data
- **THEN** each KPI card shows the signed change (e.g. "-14.9 pp MoM")

#### Scenario: Multi-month window

- **WHEN** the window spans several months
- **THEN** the cards show the aggregate for the whole window and no month-over-month change

#### Scenario: Twelve-month trend

- **WHEN** a repo is selected
- **THEN** the dashboard charts CI failure rate, PR lead time (avg, p50, p90), deployments per week, and change failure rate for the twelve months ending at the selected window, leaving gaps for months without data

### Requirement: The UI stays responsive and consistent

The user MUST be able to keep interacting while data loads, and MUST NOT see results for a previous selection replace the current one.

#### Scenario: Slow response for an old selection

- **WHEN** the user switches repo or window while an earlier request is still in flight
- **THEN** the earlier response is discarded when it arrives

#### Scenario: Selection is remembered

- **WHEN** the user restarts the dashboard
- **THEN** the last selected repo is selected again if the server still tracks it

### Requirement: Manage tracked repos from the dashboard

The user MUST be able to add, reconfigure, sync, and remove tracked repos from the dashboard's Repos page.

#### Scenario: Add a repo

- **WHEN** the user enters `owner/name` and presses Add
- **THEN** the repo appears in the list, with a message that says whether it was added, was already tracked, or was added without GitHub metadata and why

#### Scenario: Edit settings

- **WHEN** the user edits PR start or the incident / hotfix label and saves
- **THEN** only the changed fields are sent; invalid input is reported before any request, and a changed label refreshes the metrics on screen

#### Scenario: Remove needs confirmation

- **WHEN** the user presses Remove on a repo
- **THEN** the dashboard first asks to confirm that the repo and its synced data will be deleted, and deletes only after a second click

#### Scenario: Sync with progress

- **WHEN** the user starts a sync
- **THEN** the page shows the sync as running, polls until it ends, reports success or the error, and refreshes the data on screen

#### Scenario: Server cannot sync

- **WHEN** the server has no `GITHUB_TOKEN`
- **THEN** the Sync buttons are disabled and the page says to run `devpulse sync` on the server

### Requirement: Manage people and view metrics per person

The user MUST be able to map accounts to members and teams, edit the excluded accounts, and view any repo's metrics for everyone, one team, or one member.

#### Scenario: Map an unmapped account

- **WHEN** the per-member breakdown lists an account nobody has mapped and the user presses Map
- **THEN** the People page opens with a new member prefilled with that account

#### Scenario: Switch whose work is shown

- **WHEN** the user picks a member or team in the dashboard's Show picker, or presses Show on a breakdown row
- **THEN** the cards, charts, and trends cover only that work, and the DORA panel explains it is only available for everyone

#### Scenario: Scope removed underneath

- **WHEN** the member or team being shown is deleted
- **THEN** the dashboard falls back to everyone

