# Proposal

## Why

DevPulse's current metrics (build failure rate, re-push count, PR lead time, PR size) describe the PR/CI loop but say nothing about delivery performance. The four DORA metrics are the industry-standard lens for that, and most of the raw facts they need (merge timestamps, PR metadata) already flow through the PR sync — they are just not captured or aggregated. The sync also has two gaps. A PR is never refreshed after its first sync, so a later merge (or a reopen and merge) is never recorded, which blocks any merge-based metric. And GitHub Actions returns workflow runs with an empty `pull_requests` list, so no CI build is linked to its PR, and "Avg Builds per PR" always reads 0.0.

## What Changes

- Define the four DORA metrics for DevPulse, with deployment = "PR merged into the repo's default branch" and failure = "revert / hotfix PR" plus "GitHub issue carrying the incident label":
  - **Deployment Frequency** — deployments in the window, per week, and distinct deploy days.
  - **Lead Time for Changes** — earliest commit authored in the PR → merge into the default branch.
  - **Change Failure Rate** — remediation deployments (revert or hotfix PRs) ÷ deployments.
  - **Failed Deployment Recovery Time** — reverted PR merge → revert PR merge, and incident issue opened → closed.
- Extend the PR sync to capture title, labels, base/head branch, merge commit SHA, earliest commit authored time (merged PRs only), and the PR number a revert PR reverts.
- On each `repo sync`, refresh every stored open PR and every stored PR that GitHub reports as updated since the previous refresh (tracked by a per-repo `updated_at` watermark), so merges, closes and reopens after the first sync are recorded, also when the HTTP cache served an old copy.
- Link each PR-triggered CI build to its PR by head branch and open window, since the CI provider does not report it.
- Add an incident fetcher that mirrors GitHub issues carrying the repo's incident label into a new `incidents` table.
- Add per-repo settings `incident-label` (default `incident`) and `hotfix-label` (default `hotfix`) to `devpulse repo config`.
- Print a DORA section in `devpulse metrics`.

## Capabilities

### New Capabilities

- `dora-metrics`: definitions and computation of Deployment Frequency, Lead Time for Changes, Change Failure Rate, and Failed Deployment Recovery Time, and how they are reported.

### Modified Capabilities

- `vcs-data-fetching`: PR sync captures deployment/remediation facts, refreshes PRs updated upstream, and fetches incident issues.
- `ci-data-fetching`: PR-triggered builds are linked to their PR.
- `tool-configuration`: per-repo incident and hotfix label settings.

## Impact

- **Schema**: new migration adding columns to `pull_requests` (`title`, `labels`, `base_ref`, `head_ref`, `merge_commit_sha`, `first_commit_at`, `reverts_number`) and to `repos` (`incident_label`, `hotfix_label`, `pr_updated_watermark`), plus a new `incidents` table.
- **Code**: `internal/github` (PR detail fields, PR commits, issues), `internal/fetching` (watermark-based PR refresh, build-to-PR linking, incident sync, new provider/writer methods), `internal/persistence` (PR / repo / incident / metrics persisters), new pure domain package `internal/dora`, `internal/pullrequest` (revert/hotfix classification), CLI `metrics` and `repo config`.
- **API quota**: one extra REST call per merged PR (PR commits), one paginated `sort=updated` PR listing per repo per sync (it stops at the watermark) plus a detail+reviews refresh for each stored PR it lists and for each stored open PR, and one paginated issue listing per repo per sync.
- **Docs**: `README.md`, `README.zh-TW.md`, `docs/commands*.md`.
