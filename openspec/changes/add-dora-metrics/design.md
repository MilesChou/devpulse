# Design

## Context

- PR sync is a by-number backfill (`Orchestrator.BackfillPullRequestsByNumber`): it resumes at `MAX(number)+1` and never revisits a stored number. Every PR is therefore frozen at its first-sync state: an open PR's `merged_at` stays NULL forever, and so does that of a closed PR that is later reopened and merged.
- GitHub Actions workflow runs come back with an empty `pull_requests` array in practice (all 100 runs of this repo, 59 of them `pull_request` events), so `builds.pr_number` is always NULL and nothing links a build to its PR.
- The HTTP response cache (`CACHE_ENABLED`) serves any response younger than `CACHE_TTL` from disk, and a TTL of 0 never expires. Any sync cursor must therefore survive a stale listing.
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

DORA measures "commit to running in production". We fetch `GET /pulls/{n}/commits?per_page=100` once per merged PR and take the minimum `commit.author.date`. Author date survives rebases, while committer date is rewritten by them. The fetch is skipped for unmerged PRs; the PR refresh (D4) fetches it once the PR merges. PRs with more than 100 commits use the minimum over the first 100, and this limit is documented. A negative duration (author clock skew) is clamped to 0.

*Alternative*: PR `created_at` → `merged_at`. Rejected because it is the existing "PR lead time" metric and ignores the time spent coding before the PR was opened.

### D3. Remediation classification happens at metric time, in Go

Sync stores raw facts (`title`, `labels`, `head_ref`, `reverts_number`) and `internal/dora` classifies each deployment when metrics are computed:
- **Revert**: the title matches `^\s*revert(?:[\s:("!]|$)` (case-insensitive). GitHub's revert button produces `Revert "<original title>"`, and conventional commits use `revert:`, `revert(scope):` and `revert!:`. A plain `\b` is not enough: it also accepts `Revert-safe helper` and `revert/cleanup`, which would inflate the failure rate. Hand-written titles such as `Revert the login change` still count. Reverting a revert re-lands the change, and GitHub nests the titles (`Revert "Revert "X""`), so the quoted `Revert "` prefixes are peeled and only an odd nesting depth is a revert. Otherwise a re-land would inflate the failure rate and add a "recovery" sample that actually measures the time to fix the change.
- **Hotfix**: carries the repo's `hotfix-label` (case-insensitive), or its head branch starts with `hotfix/`.
- `reverts_number` is parsed from the body pattern `Reverts [owner/repo]#N` (GitHub's revert-button body). A cross-repo reference to another repository is ignored.

Classifying at query time means changing `hotfix-label` takes effect immediately, without a re-sync.

*Alternative*: store an `is_hotfix` flag at sync time. Rejected because a config change would then require re-syncing every PR.

### D4. Refresh open PRs and PRs updated upstream, tracked by an `updated_at` watermark

Before the backfill, the orchestrator refreshes the union of two sets, in ascending number order, re-running the per-PR sync for each number that is already stored and at or above `pr_sync_start_number`. Newer numbers belong to the backfill, and older ones to history the operator chose to skip. The per-PR sync is idempotent (upsert on `(repo_id, number)`), so a PR on the inclusive boundary costs one extra fetch at most.

- **Listed**: `GET /pulls?state=all&sort=updated&direction=desc`, page by page, until a PR updated before `repos.pr_updated_watermark`. This catches every kind of change, including a closed PR that is reopened and merged. On the first run there is no watermark, so only the head of the list is read (one single-item request) to seed it.
- **Open**: every PR stored as open. With the HTTP cache on (see Context), any PR detail, including one the backfill wrote during a rebuild, can be an old copy whose upstream `updated_at` is already behind the watermark, so the listing never returns it again. Refetching open PRs on every sync lets such a copy heal once its cache entry expires. With `CACHE_TTL=0` (never expire) it never heals, which is accepted because that mode is meant for replaying a rebuild.

The new watermark is the `updated_at` of the first PR in the listing, the most recently updated PR in the repo. It comes from GitHub's clock and from the same response, so neither clock skew nor a stale cached listing can move it past an update that was not returned. A PR updated while the walk is in progress jumps to the head of the list, past the pages already read; its new `updated_at` is later than the watermark, so the next sync lists it. The CLI stores the watermark as soon as the refresh returns, independent of the later sync steps.

A listed PR whose detail comes back with an `updated_at` older than the listing reported is a cached copy. It is not written, and it counts as a failure.

Error policy: a PR that fails to refresh is logged and skipped, and the watermark is not advanced, so the next sync lists it again. An upstream 404 (deleted PR) counts as done, so that a deleted PR does not pin the watermark forever. A listing failure aborts the repo sync.

*Alternative*: refresh only the PRs stored as open. Rejected because it misses a closed PR that is later reopened and merged.

*Alternative*: refresh only the listed PRs. Rejected because a stale cached open copy (above) would then never be refetched, and its later merge would be lost.

*Alternative*: use the local clock at sync start as the watermark. Rejected because, with the cache enabled, the listing can be older than the sync, and the watermark would skip past updates the cached listing never showed.

### D5. Incidents are mirrored by full replacement

`GET /repos/{o}/{r}/issues?labels=<incident-label>&state=all&per_page=100`, paginated until a short page. Items carrying a `pull_request` key are dropped. Each sync replaces the repo's `incidents` rows in one transaction (delete + insert). Incident counts are small, and replacement makes a label change or a removed label take effect without tombstones.

*Alternative*: an incremental `since=updated_at` watermark. Rejected because a label rename would leave stale rows, and incident volume does not justify the complexity.

An incident fetch failure is logged and does not fail the repo sync, which matches how a per-provider CI failure is handled.

### D6. Metric formulas (window = `[from, to)`, attributed by the event's end time)

| Metric | Formula |
|---|---|
| Deployment Frequency | `N` = deployments with `merged_at` in the window; also `N / (window days / 7)` per week, and distinct UTC deploy days. For a window that is still in progress, the window ends at now: the default window is the current month, and dividing by its full length would understate the rate early in the month. |
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
- `internal/fetching`: new `VCSProvider` methods `GetFirstCommitAt`, `ListIncidentIssues` and `ListPullRequestsUpdatedSince`, new writer methods `ListOpenNumbers`, `ReplaceForRepo` and `LinkPullRequestsByBranch`.
- `internal/persistence`: `RepoPersister.UpdatePRUpdatedWatermark`.

### D9. Link PR-triggered builds to their PR by head branch

After each build sync, one `UPDATE` fills `builds.pr_number` for every PR-triggered build that still lacks it. It uses the stored PR whose `head_ref` equals the build's branch and that was open when the build started (`pr_created_at <= started_at` and `closed_at` is NULL or `>= started_at`). A build is linked only when exactly one PR matches. Two PRs open at once from the same branch name (typically fork PRs whose head branch is `main`) stay unlinked rather than being attributed to a guess. Push builds are left alone, and a PR number reported by the CI provider is never overwritten. The pass is driven by DB state, so a later sync also links builds stored before their PR was. On this repo, all 69 PR builds matched exactly one PR.

*Alternative*: `GET /repos/{o}/{r}/commits/{sha}/pulls` per build head SHA. Rejected for now because it costs one API call per distinct SHA, and the branch match needs no extra calls and uses the `head_ref` column this change already stores.

## Risks / Trade-offs

- [Direct pushes to the default branch are invisible] → documented as a non-goal; teams that bypass PRs will see undercounted deployments.
- [Merge ≠ production deploy for teams with batched releases] → the definition is explicit in the output and the docs; another deployment source can be added later behind the same `dora` input.
- [PR refresh cost grows with upstream activity and stale open PRs] → it is bounded by the number of PRs updated since the last sync plus the number of open PRs, and the listing stops at the watermark. `pr-start` already lets operators skip ancient history.
- [Build-to-PR linking is a heuristic] → it links only unambiguous matches (D9) and never overwrites a PR number the CI provider did report.
- [Hotfix detection depends on team conventions] → the label is configurable, and the `hotfix/` branch prefix covers the other common convention.
- [Old rows lack the new columns] → documented rebuild path; the metrics output counts only rows that carry the data.

## Migration Plan

1. `devpulse migrate up` applies the new migration (memory DSNs apply it automatically).
2. `devpulse repo refresh <repo>` if `default_branch` is empty.
3. Rebuild the DB, or run `repo sync` for new data. With `CACHE_ENABLED=true` the rebuild replays cached responses.

Rollback: `devpulse migrate down` drops the added columns and the `incidents` table.
