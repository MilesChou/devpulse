# Design

## Context

- PR sync is a by-number backfill (`Orchestrator.BackfillPullRequestsByNumber`): it resumes at `MAX(number)+1` and never revisits a stored number. An open PR is therefore frozen at its first-sync state, and its `merged_at` stays NULL forever.
- The GitHub PR detail response (`GET /repos/{o}/{r}/pulls/{n}`) already carries `title`, `body`, `labels`, `base.ref`, `head.ref` and `merge_commit_sha`; `rawPull` decodes only a subset.
- `builds.is_deploy_event` exists but is always written as `false`; nothing produces deployment data today.
- The metrics layer (`MetricsPersister`) mixes SQL with in-Go aggregation (e.g. `PRLeadTime` loads rows and computes percentiles in Go).
- Decisions from the user (see proposal): deployment = merge into the default branch; failure = revert/hotfix PR and GitHub issues labelled as incidents.

## Goals / Non-Goals

**Goals:**
- Collect every raw fact the four DORA metrics need during the existing `repo sync` / `sync` run, with no new command to remember.
- Keep the metric rules in a pure, table-testable Go package so the definitions can evolve without touching SQL.

**Non-Goals:**
- GitHub Deployments API, release/tag-based deployments, or deploy workflow runs as deployment sources.
- Direct pushes to the default branch that bypass a PR (not observable through the PR sync).
- DORA performance tiers (elite / high / medium / low); only raw numbers are reported.
- Linking a hotfix PR to the specific deployment it fixed.

## Decisions

### D1. A deployment is a merged PR whose base branch is the repo's default branch

Deploy time is `merged_at`. The default branch comes from `repos.default_branch` at query time, not at sync time, so renaming the default branch only needs a `repo refresh`. If `default_branch` is empty, the DORA section reports that the default branch is unknown and points to `devpulse repo refresh`, rather than guessing.

*Alternative*: count every merged PR. Rejected because merges into release or feature branches would inflate frequency.

### D2. Lead time starts at the earliest commit **author** date in the PR

DORA measures "commit to running in production". We fetch `GET /pulls/{n}/commits?per_page=100` once per merged PR and take the minimum `commit.author.date`. Author date survives rebases, while committer date is rewritten by them. The fetch is skipped for unmerged PRs; the open-PR refresh (D4) fetches it once the PR merges. PRs with more than 100 commits use the minimum over the first 100, and this limit is documented. A negative duration (author clock skew) is clamped to 0.

*Alternative*: PR `created_at` → `merged_at`. Rejected because it is the existing "PR lead time" metric and ignores the time spent coding before the PR was opened.

### D3. Remediation classification happens at metric time, in Go

Sync stores raw facts (`title`, `labels`, `head_ref`, `reverts_number`) and `internal/dora` classifies each deployment when metrics are computed:
- **Revert**: the title matches `^revert\b` (case-insensitive). GitHub's revert button produces `Revert "<original title>"`.
- **Hotfix**: carries the repo's `hotfix-label` (case-insensitive), or its head branch starts with `hotfix/`.
- `reverts_number` is parsed from the body pattern `Reverts [owner/repo]#N` (GitHub's revert-button body). A cross-repo reference to another repository is ignored.

Classifying at query time means changing `hotfix-label` takes effect immediately, without a re-sync.

*Alternative*: store an `is_hotfix` flag at sync time. Rejected because a config change would then require re-syncing every PR.

### D4. Refresh stored open PRs after each backfill

After `BackfillPullRequestsByNumber` finishes, the orchestrator lists every stored PR with `status = 'open'` and re-runs the same per-PR sync for each one. The per-PR sync is already idempotent (upsert on `(repo_id, number)`). An upstream 404 is logged and skipped. A PR that fails to refresh is logged, and the refresh continues, because the next run retries it anyway. The cost is O(open PRs) per sync.

### D5. Incidents are mirrored by full replacement

`GET /repos/{o}/{r}/issues?labels=<incident-label>&state=all&per_page=100`, paginated until a short page. Items carrying a `pull_request` key are dropped. Each sync replaces the repo's `incidents` rows in one transaction (delete + insert). Incident counts are small, and replacement makes a label change or a removed label take effect without tombstones.

*Alternative*: an incremental `since=updated_at` watermark. Rejected because a label rename would leave stale rows, and incident volume does not justify the complexity.

An incident fetch failure is logged and does not fail the repo sync, which matches how a per-provider CI failure is handled.

### D6. Metric formulas (window = `[from, to)`, attributed by the event's end time)

| Metric | Formula |
|---|---|
| Deployment Frequency | `N` = deployments with `merged_at` in the window; also `N / (window days / 7)` per week, and distinct UTC deploy days |
| Lead Time for Changes | over deployments in the window that have `first_commit_at`: avg / p50 / p90 of `merged_at − first_commit_at`, in hours |
| Change Failure Rate | `(reverts + hotfixes) / N`; a PR that is both a revert and a hotfix counts once (as a revert) |
| Failed Deployment Recovery Time | union of (a) revert deployments in the window whose reverted PR is known and merged: `revert.merged_at − reverted.merged_at`; and (b) incidents with `closed_at` in the window: `closed_at − created_at`. avg / p50 / p90 hours, with a per-source count |

Percentiles reuse the linear-interpolation definition already used by `PRLeadTime`.

### D7. Schema changes ship as a new additive migration

Although the project is pre-release, an additive migration keeps existing databases working. It adds one `ALTER TABLE … ADD COLUMN` per statement, which is portable across SQLite, PostgreSQL and MySQL, plus the new `incidents` table. Rows synced before this change have NULL `base_ref`, so they do not count as deployments until they are re-synced. The docs recommend rebuilding the DB; the HTTP cache makes a rebuild cheap because cached PR responses already contain the new fields.

### D8. Package layout

- `internal/pullrequest`: `IsRevertTitle`, `ParseRevertedNumber`, plus the new fields on `PullRequest`.
- `internal/incident`: the `Incident` domain type.
- `internal/dora`: pure `Compute(input) Report`, with no DB and no time.Now.
- `internal/persistence`: `IncidentPersister`, `MetricsPersister.DORAInput`.
- `internal/fetching`: new `VCSProvider` methods `GetFirstCommitAt` and `ListIncidentIssues`, new writer methods `ListOpenNumbers` and `ReplaceForRepo`.

## Risks / Trade-offs

- [Direct pushes to the default branch are invisible] → documented as a non-goal; teams that bypass PRs will see undercounted deployments.
- [Merge ≠ production deploy for teams with batched releases] → the definition is explicit in the output and the docs; another deployment source can be added later behind the same `dora` input.
- [Open-PR refresh cost grows with stale open PRs] → it is bounded by the number of open PRs. `pr-start` already lets operators skip ancient history.
- [Hotfix detection depends on team conventions] → the label is configurable, and the `hotfix/` branch prefix covers the other common convention.
- [Old rows lack the new columns] → documented rebuild path; the metrics output counts only rows that carry the data.

## Migration Plan

1. `devpulse migrate up` applies the new migration (memory DSNs apply it automatically).
2. `devpulse repo refresh <repo>` if `default_branch` is empty.
3. Rebuild the DB, or run `repo sync` for new data. With `CACHE_ENABLED=true` the rebuild replays cached responses.

Rollback: `devpulse migrate down` drops the added columns and the `incidents` table.
