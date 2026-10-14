## Why

The dashboard works but reads flat: every section has the same weight, nothing says at a glance whether a number is good, and the Overview tables are wider than the window, so their last columns (review wait, deploys per week, the sparkline) are cut off. The left repo list also wraps long names such as `104corp/accounts.104.com.tw` onto two lines.

## What Changes

- **Theme**: one accent colour, rounded corners, more spacing, sections on card backgrounds, the page switcher styled as tabs; defined for both light and dark mode.
- **Overview tables fit the window**: compact wrapping headers, the 12-month sparkline next to the name, repo names without the owner prefix when it is shared, and the change against the previous period as a small coloured tag (warning colour when worse beyond the noise floor, green when better beyond it, neutral otherwise).
- **KPI status on the Dashboard**: cards with an ideal in the project goals (CI failure rate, builds per PR, PR lead time) and the small-PR share get a status colour — on target, near, or off — from thresholds around the ideal. Cards without an ideal (review wait, DORA) stay neutral.
- **Charts** use one palette derived from the accent colour.
- **Repo list grouped by owner**: the owner is a group heading and each entry shows only the repo name, with the full name on hover.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `http-api`: overview summaries report the median lead time (`lead_time_p50_hours`).

- `desktop-dashboard`: KPI cards show their status against the ideal; the repo list is grouped by owner.
- `cross-repo-overview`: the change against the previous period is also marked when it is better beyond the noise floor; the tables fit the window.

## Impact

- `desktop/src/`: a new `theme.rs` (visuals, card frame, palette), `app.rs` (layout), `kpi.rs` (status per card, thresholds, tests), `overview.rs` (better-direction marking, tests), `i18n.rs` (status wording for tooltips).
- No Go or API change.
