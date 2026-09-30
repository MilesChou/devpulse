## ADDED Requirements

### Requirement: Better changes are marked too

A change against the previous period MUST be marked as better when it goes the better way beyond the noise floor, distinct from worse and from within-the-floor changes, and every change MUST carry an arrow so colour is not the only signal.

#### Scenario: Faster lead time

- **WHEN** a repo's lead time falls from 120 h to 60 h
- **THEN** its tag reads ⏷50 % in the better colour

#### Scenario: Within the floor

- **WHEN** lead time rises from 20 h to 21 h
- **THEN** the tag is neutral

### Requirement: Overview tables fit the window

The Overview tables MUST show every column, including the sparkline, at the dashboard's default window size without horizontal scrolling.

#### Scenario: Default window

- **WHEN** the dashboard opens at its default size (1200 × 820)
- **THEN** both tables show all their columns
