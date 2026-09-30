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
