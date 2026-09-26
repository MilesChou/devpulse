# Tasks

## 1. Schema

- [x] 1.1 Add migration `20260926000001_dora` (up/down) that adds `title`, `labels`, `base_ref`, `head_ref`, `merge_commit_sha`, `first_commit_at`, `reverts_number` to `pull_requests`, adds `incident_label` / `hotfix_label` / `pr_updated_watermark` to `repos`, and creates `incidents`; verify with `go test ./internal/persistence/...` (the memory persister applies every migration, and up/down round-trips in the migrator tests)

## 2. Domain

- [x] 2.1 Add the new fields to `pullrequest.PullRequest` plus `IsRevertTitle` (odd nesting depth only, so a re-land is not a revert) / `ParseRevertedNumber`; verify with table tests in `internal/pullrequest`
- [x] 2.2 Add `repo.Repo.IncidentLabel` / `HotfixLabel` with defaults; add the `internal/incident` type; verify it builds with `go vet ./...`
- [x] 2.3 Implement `internal/dora.Compute` (frequency, lead time, CFR, recovery time) following design D6; verify with table tests covering every spec scenario in `specs/dora-metrics`

## 3. GitHub adapter

- [x] 3.1 Decode `body`, `labels`, `base.ref`, `head.ref` and `merge_commit_sha` in `rawPull` and map them (including the revert target) in `GetPullRequest`; verify with an updated `get_pull.json` fixture test
- [x] 3.2 Add `GetFirstCommitAt` (PR commits, min author date); verify with an httptest-backed test
- [x] 3.4 Add `ListPullRequestsUpdatedSince` (sort=updated, stops at since, newest from the head of the list); verify with httptest-backed tests covering the stop point, the zero-since bootstrap and an empty repo
- [x] 3.3 Add `ListIncidentIssues` (label filter, pagination, drops pull requests); verify with an httptest-backed test covering two pages and a PR item

## 4. Persistence

- [x] 4.1 Persist and scan the new PR columns in `PullRequestPersister`; add `ListOpenNumbers`; verify with persister tests
- [x] 4.2 Read and write `incident_label` / `hotfix_label` in `RepoPersister` (`UpdateLabels`) and `pr_updated_watermark` (`UpdatePRUpdatedWatermark`); verify with persister tests, including the defaults
- [x] 4.3 Add `IncidentPersister.ReplaceForRepo` / `List`; verify with persister tests (replace removes stale rows)
- [x] 4.4 Add `MetricsPersister.DORAInput` (deployments in window, reverted-PR merge times, incidents); verify with a persister test and the empty-store dialect smoke test

## 5. Orchestration

- [x] 5.1 Fetch `first_commit_at` for merged PRs in the per-PR sync; verify with an orchestrator test
- [x] 5.2 Add `RefreshPullRequests` (listed since the updated_at watermark ∪ stored open PRs, stale-detail rejection) and call it from `syncOneRepo`, storing the returned watermark; verify open→merged, closed→reopened→merged, an unlisted stale open PR healing, the stale-detail and failure watermark holds, and the number filters in orchestrator tests
- [x] 5.3 Add `SyncIncidents` and call it from `syncOneRepo` (a failure is non-fatal); verify with an orchestrator test

- [x] 5.4 Add `LinkPullRequestsByBranch` and call it from `FetchAllBuilds`; verify the branch + open-window match, ambiguity, push builds and preset numbers in a persister test, and the wiring in an orchestrator test

## 6. CLI and docs

- [x] 6.1 Add the `incident-label` / `hotfix-label` keys to `repo config`; verify with CLI tests
- [x] 6.2 Print the DORA section in `devpulse metrics`, measuring an in-progress window up to now; verify by running `devpulse metrics` against a seeded memory DB or with a CLI test
- [x] 6.3 Document the DORA definitions, the new config keys and the rebuild note in `README.md`, `README.zh-TW.md`, `docs/commands.md` and `docs/commands.zh-TW.md`; verify both READMEs carry the same sections

## 7. Integration

- [x] 7.1 Run `make all` (with `GITHUB_TOKEN` unset, since the CLI tests assume none) and `openspec validate add-dora-metrics --strict`; both pass
