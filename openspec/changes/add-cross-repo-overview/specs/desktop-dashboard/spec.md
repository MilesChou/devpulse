## ADDED Requirements

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
