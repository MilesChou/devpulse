## Context

Built on people mapping (PR #34, branch `claude/people-mapping`). Today every metric is per repo:

- `metrics.Source` (implemented by `persistence.MetricsPersister`) takes one `repoID` per query; the SQL filters `repo_id = ?`.
- `MetricsPersister.Scoped(accounts)` narrows the same queries to a member's or team's accounts and always drops excluded accounts (bots).
- `metrics.Compute` builds one `Report` for one repo and window; `ComputeMonthly` runs it once per month; `ComputeByMember` runs it once per active member of one repo.
- The dashboard selects one repo in the left list, then optionally a member or team in **Show**.

Averages and percentiles cannot be combined after the fact: the p50 lead time of two repos is not a function of their two p50s. Cross-repo numbers therefore have to come from queries over the pooled rows.

## Goals / Non-Goals

**Goals:**

- Metrics over all tracked repos, for everyone or one member / team, with the same `Report` shape and the same definitions as per-repo metrics.
- Comparison rows for repos and for members: current period, previous period of equal length, 12 monthly summaries for a sparkline.
- On the dashboard: an Overview page to compare and drill down, and an "All repos" target so a member can be picked first.

**Non-Goals:**

- Team comparison rows. The API scope `team=` keeps working on the cross-repo report, but the Overview has members only.
- DORA across repos. A deployment is a merge into one repo's default branch; pooling repos with different release practices yields a number nobody acts on. `dora` is null on cross-repo reports.
- Choosing a subset of repos. "All repos" means every tracked, enabled repo.
- Caching. Revisit if the measured timings in the tasks are too slow.

## Decisions

### 1. Queries take a repo set

`Source` methods change from `repoID string` to `repoIDs []string`, rendered as `repo_id IN (?, …)` (the variable-length `IN` pattern `ownerFilter` already uses). A per-repo call passes one id, so per-repo SQL keeps its meaning and its tests. `DORAInput` keeps a single repo.

*Alternative:* compute per repo and merge. Rejected: percentiles and weighted averages (builds per PR, review wait) would be wrong or need raw samples returned from every query.

### 2. A target instead of a repo

`metrics.Compute` takes a `Target` — either one `repo.Repo` or `All(repos)` — instead of a `repo.Repo`. `Report.Repo` is `owner/name` for a repo and `*` for all repos; DORA is computed only for a single repo. `ComputeMonthly` and the breakdown follow the same target.

### 3. Summaries for comparison rows

Comparison rows do not carry full reports. A `Summary` has the columns the tables show, each `null` when there is no data (the dashboard shows "—", never a misleading 0):

| Field | From | Better |
|---|---|---|
| `prs_opened` | PR size distribution total | neutral |
| `prs_merged` | PR lead time count | neutral |
| `lead_time_hours` | PR lead time avg | lower |
| `builds_per_pr` | avg builds per PR | lower |
| `ci_failure_rate` | build failure rate | lower |
| `avg_build_seconds` | daily build duration, weighted by build count | lower |
| `review_wait_hours` | review wait avg | lower |
| `deploys_per_week` | DORA, repo rows only | higher |

`avg_build_seconds` is new: it answers "which repo got slower" directly, where the daily chart only shows the shape. It is derived from the report's daily series (each day carries its average and build count), so `Report` and the per-repo responses do not change.

### 4. Endpoints

```
GET /api/v1/metrics?from&to[&member|&team]          → Report (repo "*", dora null)
GET /api/v1/metrics/monthly?from&to[&member|&team]  → MonthlyReport, 120-month limit
GET /api/v1/overview/repos?from&to                  → Overview
GET /api/v1/overview/members?from&to                → Overview
```

```json
{
  "from": "2026-09", "to": "2026-10",
  "previous": {"from": "2026-08", "to": "2026-09"},
  "rows": [{
    "repo": "owner/name",             // repo rows
    "member_id": "01M…", "name": "…", // member rows; member_id null for an unmapped account
    "accounts": ["…"],
    "current": Summary, "previous": Summary,
    "monthly": [{"month": "2025-10", "summary": Summary}, …]   // 12, ending at `to`
  }]
}
```

The previous period is the same number of months immediately before `from`. Member rows are members with activity in the current or previous period across all repos, then active accounts no member claims, as in `/metrics/by-member`.

### 5. Performance budget

One overview row costs 2 (current, previous) + 12 (monthly) reports of about 7 queries each. Estimates, to be replaced by measurements in the tasks: 6 repos → ~600 queries; 25 members → ~2,500 queries. Rows are computed with a bounded worker pool (4), which SQLite serves concurrently for reads. If the measured members overview on the synced test data exceeds 10 s, the monthly summaries switch to grouped queries (`GROUP BY` month) before shipping.

### 6. Dashboard

- **Overview page** (first in the page switcher, and the start page): repo table on top, member table below, both for the top bar's period. Headers sort on click (again to reverse); the row's sparkline follows the sorted column. A change in the worse direction is coloured when it exceeds a noise floor: 10 % relative, or 2 percentage points for rates.
- **Drill down**: a repo row selects that repo on the Dashboard; a member row selects **All repos** plus that member; an unmapped account row offers **Map…** as the by-member table does.
- **All repos** is the first entry of the repo list. The settings file stores it as `last_repo: "*"`. With All repos the By member table is replaced by a link to the Overview, which is the cross-repo breakdown, and the DORA panel says to pick a repo.
- The sparkline is painted with egui shapes, not `egui_plot`, so a table of 30 rows stays cheap.
- Every string is in the i18n table in English and Traditional Chinese.

### Measured (task 3.4)

On the synced test data (4 repos, 19,158 builds, 4,951 PRs, SQLite, Apple Silicon), release build of the server:

| Request | Time | Rows |
|---|---|---|
| `overview/repos`, one month | 0.20 s | 4 |
| `overview/members`, one month | 0.34 s | 4 |
| `overview/repos`, 33 months | 0.48 s | 4 |
| `overview/members`, 33 months | 1.71 s | 13 |
| `metrics` across repos, one month | 0.02 s | — |

Well under the 10 s threshold, so the monthly summaries keep using the per-month reports; no grouped queries.

## Risks / Trade-offs

- [Overview requests are the heaviest the server serves] → measure on the synced test data (~6 repos, ~20k builds); fall back to grouped monthly queries; the dashboard fetches the overview once per period change, not per frame.
- [Changing `Source` touches every metric query and its tests] → mechanical; per-repo callers pass a one-element set, and the existing per-repo golden files must not change.
- [A member active in many repos pools very different CI setups] → accepted: that is the question being asked; the per-repo dashboard remains for detail.
- [Noise floor hides small real regressions] → the value and the change are always shown; only the colour depends on the floor.

## Migration Plan

No schema change. `last_repo: "*"` is a new value an older build reads as an unknown repo and ignores.

## Open Questions

- Should the Overview remember its sort column across restarts? Default: no.
- Include disabled repos in the repo table? Default: no, as `devpulse sync` skips them.
