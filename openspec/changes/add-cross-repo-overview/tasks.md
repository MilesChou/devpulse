## 1. Metrics over a repo set

- [x] 1.1 Change `metrics.Source` and `persistence.MetricsPersister` queries from `repoID string` to `repoIDs []string` (`repo_id IN (…)`); keep `DORAInput` per repo; per-repo callers pass one id
- [x] 1.2 Persister tests: pooled lead-time percentiles across two repos (the spec example), and every existing per-repo test unchanged
- [x] 1.3 Derive a build-weighted average build duration (`avg_build_seconds`) from the report's daily series, in `Summary` (task 2.1), with the spec's weighted-average example as a test; `Report` itself is unchanged so per-repo responses and their golden files stay the same
- [x] 1.4 Introduce `metrics.Target` (one repo or all repos); `Compute` / `ComputeMonthly` take it; `Report.repo` is `*` and `dora` is null for all repos
- [x] 1.5 Extend the member breakdown's active-author listing to a repo set

## 2. Overview assembly

- [x] 2.1 `metrics.Summary` with the design's nullable fields, derived from a `Report`
- [x] 2.2 Previous period of equal length; unit-test 1-month and 3-month windows and a window starting in January
- [x] 2.3 `ComputeRepoOverview` and `ComputeMemberOverview`: current, previous and 12 monthly summaries per row, computed with a bounded worker pool (4)
- [x] 2.4 Member rows: members active in the current or previous period across all repos, then unmapped active accounts; excluded accounts never appear

## 3. HTTP API

- [x] 3.1 `GET /api/v1/metrics` and `GET /api/v1/metrics/monthly` across all repos, with `member` / `team` scope and the 120-month limit
- [x] 3.2 `GET /api/v1/overview/repos` and `GET /api/v1/overview/members`
- [x] 3.3 Golden files for the new responses; existing per-repo golden files unchanged
- [x] 3.4 Measure both overview endpoints against the synced test data (≈6 repos); if the members overview takes over 10 s, switch monthly summaries to grouped queries; record the timings in design.md

## 4. Desktop: data and state

- [x] 4.1 `api.rs`: types and client calls for the four endpoints; decode the new golden files in tests
- [x] 4.2 `state.rs`: the selected target is one repo or All repos; `last_repo: "*"` restores All repos; Overview state (rows, loading, errors) keyed by period
- [x] 4.3 Sorting: per-table sort column and direction, worst first, rows without data last; unit-test with fixture rows
- [x] 4.4 Change marking: worse direction per column and the noise floor (2 pp for rates, 10 % otherwise); unit-test each column's direction

## 5. Desktop: UI

- [x] 5.1 Page switcher Overview | Dashboard | Repos | People, opening on Overview
- [x] 5.2 Overview page: repo table and member table with value, change and sparkline cells; clickable sortable headers; (i) help on each column
- [x] 5.3 Sparkline painted with egui shapes, gaps for months without data
  - note: sort arrows use ⏷ / ⏶ from egui's bundled icon font; ▲▼ are missing there. The same test showed the top bar's ◀ ▶ were missing too (they only rendered through the CJK fallback font), so they now use ⏴ / ⏵.
- [x] 5.4 Drill down: repo row → Dashboard for that repo; member row → Dashboard for All repos and that member; unmapped row → Map…
- [x] 5.5 "All repos" at the top of the repo list; DORA panel message; By member replaced by a link to the Overview
- [x] 5.6 Every new string in `i18n.rs`, English and Traditional Chinese

## 6. Verify and document

- [x] 6.1 `make all` equivalents (`go vet`, `go test ./...`) and `make desktop-lint desktop-test` pass
- [x] 5.7 Unmapped accounts can be opened too: the API accepts `account=<login>` as a scope (per repo and across repos), and clicking an unmapped name opens the Dashboard for All repos and that account; found when the user could not click any member because none was mapped yet
- [ ] 6.2 Run against the synced test data: Overview for 2026-09 and for a multi-year period; sort each column; drill down into a repo and a member; restart and confirm All repos is restored
- [x] 6.3 `docs/commands.md` / `docs/commands.zh-TW.md` (new endpoints) and `desktop/README.md` / `desktop/README.zh-TW.md` (Overview, All repos) in sync
- [x] 6.4 `openspec validate add-cross-repo-overview --strict`
