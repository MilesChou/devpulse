## ADDED Requirements

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

## MODIFIED Requirements

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

