# Desktop Dashboard

## Purpose

A native desktop app (`desktop/`, Rust) that charts the metrics served by the [HTTP API](../http-api/spec.md) and manages which repos the server tracks. Data collection and every GitHub / CI credential stay with the Go service.
## Requirements
### Requirement: Connect with a server URL and an API token only

The user MUST be able to use the dashboard by entering the DevPulse server URL and, when the server requires one, its API token, without installing a database driver or handing the dashboard any GitHub or CI token. The dashboard MUST NOT refuse to connect because no token was entered; whether a token is required is the server's decision.

#### Scenario: First launch

- **WHEN** the user starts the dashboard with no stored token
- **THEN** the settings panel opens with the server URL and the optional API token, and the dashboard connects to the server without a token

#### Scenario: Server without a token

- **WHEN** the user connects with the token field empty to a server that has no `DEVPULSE_API_TOKEN`
- **THEN** the dashboard sends no `Authorization` header and loads the repos

#### Scenario: Server requires a token

- **WHEN** the user connects with the token field empty to a server that requires a token
- **THEN** the dashboard shows the server's 401 as a message saying the token was rejected or is required

#### Scenario: Connection test

- **WHEN** the user presses Test
- **THEN** the dashboard reports whether the server is reachable and whether it accepts the connection, as separate failure messages

### Requirement: The API token is kept in the OS keychain

The user MUST NOT find the API token in a plain-text settings file; it is stored in the operating system's credential store, per server URL.

#### Scenario: Token storage

- **WHEN** the user saves a token
- **THEN** it is written to the OS keychain (macOS Keychain, Windows Credential Manager, Secret Service), and the settings file holds only the server URL, the last selected repo, and the chosen UI language

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

The user MUST be able to see whether each metric improved against the previous month and how it moved over the last year, or over the whole selected window when that is longer.

#### Scenario: Change against the previous month

- **WHEN** a single month is selected and the previous month has data
- **THEN** each KPI card shows the signed change (e.g. "-14.9 pp MoM")

#### Scenario: Multi-month window

- **WHEN** the window spans several months
- **THEN** the cards show the aggregate for the whole window and no month-over-month change

#### Scenario: Twelve-month trend

- **WHEN** a repo is selected and the window spans twelve months or fewer
- **THEN** the dashboard charts CI failure rate, PR lead time (avg, p50, p90), deployments per week, and change failure rate for the twelve months ending at the selected window, leaving gaps for months without data

#### Scenario: Trend over a long window

- **WHEN** the window spans more than twelve months
- **THEN** the trend charts cover the whole window, one point per month, with axis labels thinned out so they do not overlap

#### Scenario: Trend charts share one month axis

- **WHEN** some months of the trend have no data for a chart (e.g. no PR builds before a repo adopted CI)
- **THEN** the chart's x axis still spans every month of the trend, leaving those months blank, so all trend charts line up month for month

#### Scenario: Trend beyond the server's limit

- **WHEN** the window spans more than 120 months, the most the server returns in one monthly trend
- **THEN** the trend charts cover the last 120 months of the window, and the trend heading shows that range

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

### Requirement: Each chart and KPI card explains what it shows

Next to every chart title and every KPI or DORA card title the dashboard MUST show an info icon that explains which data is counted, how it is computed, and which direction or value is better.

#### Scenario: Hovering the info icon

- **WHEN** the user hovers the info icon next to a chart or card title
- **THEN** a tooltip in the UI language explains it, e.g. that the PR size distribution counts PRs opened in the window by total changed lines, with the XS–XL thresholds

#### Scenario: Clicking the info icon

- **WHEN** the user clicks the info icon
- **THEN** the explanation stays open in a popup until the user clicks outside it

#### Scenario: Icon without special fonts

- **WHEN** no font on the machine has an info glyph
- **THEN** the icon still renders, because it is drawn rather than typed

### Requirement: Drag across a trend chart to pick months

The user MUST be able to set the period by dragging across a trend chart, as in Grafana.

#### Scenario: Selecting a range

- **WHEN** the user drags across a trend chart from one month to another
- **THEN** the selected whole months are highlighted while dragging, and on release the period becomes those months and every metric reloads

#### Scenario: Discoverability

- **WHEN** trend charts are shown
- **THEN** a caption under the trend heading says that dragging sets the period

### Requirement: Start from a person across all repos

The user MUST be able to pick a person first and see their work across every repo, without picking a repo.

#### Scenario: All repos

- **WHEN** the user selects "All repos" at the top of the repo list and a member in Show
- **THEN** the cards, charts and trends cover that member's work in every tracked, enabled repo

#### Scenario: DORA needs a repo

- **WHEN** All repos is selected
- **THEN** the DORA panel says DORA is measured per repo and asks to pick one, instead of showing numbers

#### Scenario: Breakdown lives on the Overview

- **WHEN** All repos is selected for everyone
- **THEN** the By member table is replaced by a link to the Overview, whose member table is the cross-repo breakdown

#### Scenario: Remembered

- **WHEN** the user restarts the dashboard after selecting All repos
- **THEN** All repos is selected again

### Requirement: Overview is the start page

The dashboard MUST open on the Overview page, with the page switcher listing Overview, Dashboard, Repos and People in that order.

#### Scenario: Launch

- **WHEN** the user starts the dashboard with a working connection
- **THEN** the Overview page is shown for the current month

### Requirement: KPI cards show their status against the ideal

A KPI card whose metric has an ideal in the project goals MUST show whether the value is on target, near, or off, in colour and in words.

#### Scenario: Off target

- **WHEN** the CI failure rate for the window is 22 %
- **THEN** the CI failure rate card is marked off target, and its tooltip says so

#### Scenario: On target

- **WHEN** PR lead time averages 18 h
- **THEN** the PR lead time card is marked on target

#### Scenario: No ideal, no status

- **WHEN** a card has no ideal in the project goals (review wait, the DORA cards) or has no data
- **THEN** it has no status colour

### Requirement: Repo list grouped by owner

The repo list MUST group repos under their owner and show each repo by its name alone, so long names do not wrap.

#### Scenario: Grouping

- **WHEN** the server tracks `104corp/signin.104.com.tw` and `MilesChou/devpulse`
- **THEN** the list shows "All repos", then an `104corp` group with `signin.104.com.tw` and a `MilesChou` group with `devpulse`, and hovering an entry shows its full name

#### Scenario: Collapsing

- **WHEN** the user folds an owner group, or hides the whole list with the button at the left of the top bar
- **THEN** that group's repos, or the list, are hidden to give the page more room; whether the list is hidden is remembered across restarts

### Requirement: Common periods in one click

The top bar MUST offer this month, last month, this year, the last 12 months and last year as one-click periods, and show which one the current period is.

#### Scenario: Picking a preset

- **WHEN** the current month is 2026-09 and the user picks "Last 12 months"
- **THEN** the period becomes 2025-10 to 2026-09 and every page reloads for it

#### Scenario: Other presets

- **WHEN** the current month is 2026-09
- **THEN** this month is 2026-09, last month 2026-08, this year 2026-01 to 2026-12 (the whole calendar year; months not yet reached are empty), and last year 2025-01 to 2025-12

#### Scenario: Custom period

- **WHEN** the period was typed in and matches no preset
- **THEN** the picker reads "Custom"

### Requirement: Lead times lead with the median

Duration cards (PR open to merge, DORA commit to deploy, recovery time) MUST show the median as the headline, with the mean and p90 as detail, and compare months and judge the status by the median, so a few PRs left open for weeks do not dominate.

#### Scenario: Long tail

- **WHEN** PRs merged in the window take a median of 20 h and a mean of 100 h
- **THEN** the card shows 20.0h, the detail reads "avg 100.0h", and the card is on target against the 24 h ideal

#### Scenario: Plain names

- **WHEN** the UI language is Traditional Chinese
- **THEN** the PR duration is named "PR 開啟到合併" and DORA's Lead Time for Changes "commit 到部署", since "前置時間" does not say what is measured

### Requirement: Trend points show their value on hover

Hovering a data point of a trend line MUST show the month, the series and the value with its unit.

#### Scenario: Hover a point

- **WHEN** the user hovers the 2026-07 point of the CI failure rate trend
- **THEN** a label shows 2026-07, the series name and the rate, e.g. "9.4%"

### Requirement: Median build time at a readable granularity

The build time chart MUST show the median build duration: one bar per UTC day for a single month, and one bar per month for a longer period, where daily bars are too many to read. Its title says it is the median.

#### Scenario: Single month

- **WHEN** the period is one month
- **THEN** the chart "Median build time" has one bar per day with builds, each the median of that day's builds

#### Scenario: Several months

- **WHEN** the period is 2026-01 to 2026-12
- **THEN** the chart has one bar per month with builds, each the median of that month's builds

### Requirement: Hover labels are never clipped

Charts MUST leave room above the tallest bar so its hover label stays visible.

#### Scenario: Tallest bar

- **WHEN** the user hovers the tallest bar of the PR size distribution
- **THEN** its label is fully visible

