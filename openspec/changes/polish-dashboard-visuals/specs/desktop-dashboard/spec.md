## ADDED Requirements

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

