## Why

Every view starts from one repo: pick a repo, then (since people mapping) optionally narrow it to a member. Two questions the project exists to answer cannot be asked that way: "how is this person doing across everything they touch, and how do people compare?" and "which repo got slower or started failing more lately?". Both need metrics that span repos and a place to compare rows side by side.

## What Changes

- **Overview page** in the desktop dashboard, first in the page switcher (Overview | Dashboard | Repos | People):
  - A **repo comparison** table, one row per tracked repo, and a **member comparison** table, one row per member plus active accounts no member claims yet, both for the selected period.
  - Each metric shows its change against the previous period of the same length; a change in the worse direction is highlighted.
  - Columns are sortable. Each row has a 12-month sparkline of the column it is sorted by.
  - Clicking a row opens that repo's dashboard, or that member's dashboard across all repos.
- **"All repos"** entry at the top of the dashboard's repo list. With a member picked in **Show**, it is the person-first view: the member's cards, charts and trends across every repo. DORA stays per repo, so the DORA panel explains it needs a single repo.
- **Go API**, all read-only and behind the existing token:
  - `GET /api/v1/metrics` and `GET /api/v1/metrics/monthly`: the existing report shape across all tracked repos, with the existing `member` / `team` scope; `dora` is null.
  - `GET /api/v1/overview/repos` and `GET /api/v1/overview/members`: comparison rows with current-period, previous-period and 12 monthly summaries each.
  - Bot exclusion and the 120-month trend limit apply unchanged.
- **Metrics layer**: the queries take a set of repos instead of one, so cross-repo averages and percentiles (p50 / p90) are computed over the pooled rows, not averaged from per-repo reports.
- Out of scope: team comparison (members only for now), cross-repo DORA.

## Capabilities

### New Capabilities

- `cross-repo-overview`: comparing repos and members for a period, with previous-period change and monthly sparklines, in the API and on the dashboard's Overview page.

### Modified Capabilities

- `metrics-aggregation`: metrics can be computed over a set of repos, pooling the underlying rows.
- `http-api`: cross-repo metrics endpoints alongside the per-repo ones.
- `desktop-dashboard`: the repo list gains "All repos"; the dashboard can show a member's work across repos.

## Impact

- **Go**: `internal/persistence/persister_metrics.go` (repo filter becomes a set: `repo_id IN (…)`), `internal/metrics` (`Source` signature, `Compute` over a repo set, new overview assembly), `internal/http` (new routes, golden files), `internal/metrics/breakdown.go` (authors across repos).
- **Desktop**: `api.rs` (new endpoints and types), `state.rs` (Overview state, selected target becomes one repo or all), `app.rs` (Overview page, sortable tables, sparklines, All repos entry), `i18n.rs` (all new strings in English and Traditional Chinese), `kpi.rs` unchanged.
- **Performance**: an overview request runs one report per row for the current and previous periods and one per month for sparklines; design.md sets how this stays within the server's 30 s write timeout.
- **Docs**: `docs/commands*.md` (API section), `desktop/README*.md`, both in sync.
- **Depends on** people mapping (PR #34, branch `claude/people-mapping`), which this change is built on.
