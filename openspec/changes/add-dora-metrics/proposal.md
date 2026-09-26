# Proposal

## Why

DevPulse's current metrics (build failure rate, re-push count, PR lead time, PR size) describe the PR/CI loop but say nothing about delivery performance. The four DORA metrics are the industry-standard lens for that, and most of the raw facts they need (merge timestamps, PR metadata) already flow through the PR sync — they are just not captured or aggregated. The sync also has a gap that blocks any merge-based metric: a PR that is still open when first synced is never refreshed, so its later merge is never recorded.

## What Changes

- Define the four DORA metrics for DevPulse, with deployment = "PR merged into the repo's default branch" and failure = "revert / hotfix PR" plus "GitHub issue carrying the incident label":
  - **Deployment Frequency** — deployments in the window, per week, and distinct deploy days.
  - **Lead Time for Changes** — earliest commit authored in the PR → merge into the default branch.
  - **Change Failure Rate** — remediation deployments (revert or hotfix PRs) ÷ deployments.
  - **Failed Deployment Recovery Time** — reverted PR merge → revert PR merge, and incident issue opened → closed.
- Extend the PR sync to capture title, labels, base/head branch, merge commit SHA, earliest commit authored time (merged PRs only), and the PR number a revert PR reverts.
- Refresh every stored open PR on each `repo sync` so merges that happen after the first sync are recorded.
- Add an incident fetcher that mirrors GitHub issues carrying the repo's incident label into a new `incidents` table.
- Add per-repo settings `incident-label` (default `incident`) and `hotfix-label` (default `hotfix`) to `devpulse repo config`.
- Print a DORA section in `devpulse metrics`.

## Capabilities

### New Capabilities

- `dora-metrics`: definitions and computation of Deployment Frequency, Lead Time for Changes, Change Failure Rate, and Failed Deployment Recovery Time, and how they are reported.

### Modified Capabilities

- `vcs-data-fetching`: PR sync captures deployment/remediation facts, refreshes open PRs, and fetches incident issues.
- `tool-configuration`: per-repo incident and hotfix label settings.

## Impact

- **Schema**: new migration adding columns to `pull_requests` (`title`, `labels`, `base_ref`, `head_ref`, `merge_commit_sha`, `first_commit_at`, `reverts_number`) and to `repos` (`incident_label`, `hotfix_label`), plus a new `incidents` table.
- **Code**: `internal/github` (PR detail fields, PR commits, issues), `internal/fetching` (open-PR refresh, incident sync, new provider/writer methods), `internal/persistence` (PR / repo / incident / metrics persisters), new pure domain package `internal/dora`, `internal/pullrequest` (revert/hotfix classification), CLI `metrics` and `repo config`.
- **API quota**: one extra REST call per merged PR (PR commits), one detail+reviews refresh per stored open PR per sync, and one paginated issue listing per repo per sync.
- **Docs**: `README.md`, `README.zh-TW.md`, `docs/commands*.md`.
