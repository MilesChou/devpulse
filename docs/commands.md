# DevPulse CLI Commands

DevPulse follows a **noun-verb** layout (similar to `gh` and `jira-cli`):

```
devpulse <noun> <verb> [arguments] [flags]
```

## Prerequisites

All commands except `migrate` require the following environment variables to be set (see `.env.example`):

| Variable | Description |
|---|---|
| `DEVPULSE_DSN` | Database connection string (PostgreSQL, MySQL, SQLite, or `memory`) |
| `GITHUB_TOKEN` | GitHub personal access token (`repo` + `read:user` scopes) |
| `TRAVIS_TOKEN` | *(optional)* Travis CI API token. When set, `sync` / `repo sync` also pull Travis builds in addition to GitHub Actions runs |

Optional HTTP response cache (off by default):

| Variable | Description |
|---|---|
| `CACHE_ENABLED` | `true` to cache successful API responses on disk |
| `CACHE_DIR` | Cache directory (default: `os.UserCacheDir()/devpulse`) |
| `CACHE_TTL` | Freshness window (default `24h`). Entries past the TTL revalidate with ETag / If-Modified-Since — a 304 costs no GitHub rate limit. **`CACHE_TTL=0` is replay mode**: entries never expire, so a DB rebuild can re-ingest everything without network access — but upstream changes are invisible. Never use `0` for routine syncing |

## Command Reference

### `sync`

```
devpulse sync
```

Runs `repo sync` against every repository in the store, sequentially. This is the entry point intended for cron / CI: register your repos once with `repo add`, then schedule `devpulse sync` on whatever cadence you need.

- **Disabled repos are skipped** (with a `skipped <owner/name> (disabled)` line); a `disabled = true` flag means GitHub has archived or disabled the repo upstream, so syncing it would waste API quota and almost always error.
- **Per-repo failures do not abort the loop.** A failure is logged inline (`failed <owner/name>: <err>`), recorded, and the next repo runs. After the loop a summary line (`sync: synced=N skipped=M failed=K`) is printed, followed by a list of every failure for easy grep.
- **Exit code is non-zero if any repo failed**, so cron and CI can treat the command's status as the batch's overall health.
- **Sequential, not parallel.** GitHub and Travis both rate-limit per-token; parallelism would bunch the burn without buying meaningful throughput. To sync a single repo on demand, use `repo sync` directly.

`GITHUB_TOKEN` is required and validated before the database is opened (fail-fast). `TRAVIS_TOKEN` is optional — without it only GitHub Actions builds are synced.

**Arguments**

None.

**Output**

```
Synced MilesChou/devpulse pull requests: written=7
Synced MilesChou/devpulse ci builds: written=42
skipped acme/legacy (disabled)
failed acme/broken: sync pull requests: github: 404 Not Found

sync: synced=1 skipped=1 failed=1
failures:
  acme/broken: sync pull requests: github: 404 Not Found
```

**Example**

```sh
devpulse sync
```

---

### `repo add`

```
devpulse repo add <owner/name>
```

Registers a GitHub repository in the DevPulse store. If the repository is already registered, the existing record is returned unchanged (idempotent).

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug, e.g. `MilesChou/devpulse` |

**Output**

```
MilesChou/devpulse (id=01J5HQ...)
```

**Example**

```sh
devpulse repo add MilesChou/devpulse
```

---

### `repo remove`

```
devpulse repo remove <owner/name> --yes
```

Stops tracking a repository and deletes every pull request, review, build and incident synced for it, in one transaction. Registering it again later re-syncs from scratch. Without `--yes` the command refuses, since the deletion cannot be undone.

**Output**

```
Removed MilesChou/devpulse
```

---

### `repo config set` / `repo config get`

```
devpulse repo config set <owner/name> <key> <value>
devpulse repo config get <owner/name> [key]
```

Read or write per-repo operator settings. Settings are operator-owned and are **not** overwritten by `repo sync` — once set they persist until the next `config set`.

**Available keys**

| Key | Type | Description |
|---|---|---|
| `pr-start` | integer (>= 1) | Minimum PR number the by-number sync will probe. Default `1` (full history). Bump it to skip early PRs that predate CI on a repo, saving GitHub API quota. |
| `incident-label` | string (non-blank) | Issue label that marks an incident. Default `incident`. Every issue carrying it is mirrored on each sync and feeds DORA recovery time. Takes effect on the next sync. |
| `hotfix-label` | string (non-blank) | PR label that marks a hotfix. Default `hotfix`. Counts toward DORA change failure rate. Applied at `metrics` time, so no re-sync is needed. |

Labels match case-insensitively.

The repo must already be registered with `devpulse repo add`. With no key argument, `repo config get` prints every known setting.

**Example**

```sh
# Skip PRs #1..#499 (e.g. early no-CI history) and start syncing at #500.
devpulse repo config set MilesChou/devpulse pr-start 500

# Read it back.
devpulse repo config get MilesChou/devpulse pr-start
# → 500

# Use your team's labels for DORA.
devpulse repo config set MilesChou/devpulse incident-label sev-1
devpulse repo config set MilesChou/devpulse hotfix-label urgent-fix

# Or print every setting.
devpulse repo config get MilesChou/devpulse
# → pr-start=500
# → incident-label=sev-1
# → hotfix-label=urgent-fix
```

---

### `repo sync`

```
devpulse repo sync <owner/name>
```

Syncs the repository in five steps, in order:

1. **PR refresh** — re-fetches every stored PR that GitHub reports as updated since the previous refresh, plus every PR stored as open, so a merge, close, reopen, retitle or relabel after its first sync is recorded (merges are DORA deployments). The high-water mark is the `updated_at` of the most recently updated PR seen, taken from GitHub's own listing, so a cached listing can never skip updates. A PR whose detail comes back older than the listing (a cached copy) is not written. A PR that fails to refresh is logged, and the mark is not advanced so the next sync lists it again. Open PRs are re-fetched every time so that a stale cached copy, for example one written during a rebuild with `CACHE_ENABLED=true`, heals once its cache entry expires.
2. **Pull requests** — fetches all new PRs from GitHub (detail, reviews, and — for merged PRs — the earliest commit time), upserts them, and runs enrichment. A failure to fetch the earliest commit time does not fail the PR; step 3 retries it. Commit author dates before 1971 (broken clocks) are ignored, and one later than the merge is stored as the merge time.
3. **PR fact completion** — re-fetches every stored PR that is missing its DORA facts: no base branch (a row written before the DORA columns existed) or merged with no earliest commit time. This is how a store synced before DORA support fills in its history. It walks ascending, stops at the first failure with a warning, and resumes there on the next sync; it never fails the command.
4. **CI builds** — fetches build records from every registered CI provider (GitHub Actions always; Travis CI when `TRAVIS_TOKEN` is set) and upserts them. GitHub Actions leaves a run's PR list empty in practice, so each PR-triggered build is then linked to the stored PR whose head branch matches the build's branch and that was open when the build started. A build that matches two PRs (for example, two fork PRs from branches named `main` open at once) stays unlinked.
5. **Incidents** — mirrors every issue carrying the repo's `incident-label` (open and closed; pull requests excluded). A failure prints a warning and does not fail the command.

If step 1, 2 or 4 fails, later steps are skipped and the command exits non-zero. `GITHUB_TOKEN` is required; `TRAVIS_TOKEN` is optional.

> The first run is the expensive one: PR sync walks PR numbers ascending from `pr_sync_start_number` (default 1) up to the upstream max, fetching detail + reviews per PR; build sync walks each provider's full history with no page cap (cold-start path triggered when that provider has no rows yet). Subsequent runs are incremental — PRs resume from `MAX(number) + 1`, and each CI provider resumes from its **own** `MAX(started_at) - 6h` watermark (per-provider cursors keep a lagging or newly added provider from inheriting another's progress; the 6-hour overlap absorbs retry builds and runs that were still executing at the previous sync — 6h is the GitHub Actions per-job hard timeout — while the `(repo_id, ci_provider, external_id)` unique dedupes anything already on file). Author back-fill only touches commit SHAs whose author is still NULL.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |

**Output**

```
Refreshed MilesChou/devpulse pull requests: 3
Synced MilesChou/devpulse pull requests: written=7
Completed MilesChou/devpulse pull request facts: 120
Synced MilesChou/devpulse ci builds: written=42
Synced MilesChou/devpulse incidents (label "incident"): 2
```

**Example**

```sh
devpulse repo sync MilesChou/devpulse
```

---

### `pr sync`

```
devpulse pr sync <owner/name> <number>
```

Re-fetches detail and review data for a single pull request that is already in the store, then writes the enrichment patch. Use this to refresh a specific PR without re-syncing the entire repository.

The PR must already exist in the store. If it does not, run `devpulse repo sync` first.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |
| `number` | Pull request number, e.g. `42` |

**Output**

```
Synced MilesChou/devpulse#42
```

**Example**

```sh
devpulse pr sync MilesChou/devpulse 42
```

---

### `metrics`

```
devpulse metrics <owner/name> [--from YYYY-MM] [--to YYYY-MM]
```

Prints the engineering-efficiency metrics for a repo over a month window: CI failure rate (PR builds only), average builds per PR, PR lead time (avg / p50 / p90), review wait time, PR size distribution, daily average build duration, and the four DORA metrics (see [DORA definitions](#dora-definitions)).

`--from` defaults to the current month; `--to` is exclusive and defaults to one month after `--from`.

Work by **excluded accounts** (bots; see [People](#people)) is left out of every metric except DORA: their PRs, their builds (a build's owner is its PR's author, else its commit author), and their reviews. Review wait is measured to the first review by a non-excluded account, so a Copilot review a minute after "ready" no longer makes a PR look reviewed instantly. The defaults exclude `dependabot`, `github-actions` and `copilot-pull-request-reviewer`.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |

**Flags**

| Flag | Description |
|---|---|
| `--from` | Start month, inclusive (`YYYY-MM`; default: current month) |
| `--to` | End month, exclusive (`YYYY-MM`; default: `--from` + 1 month) |

**Output**

```
Metrics for MilesChou/devpulse (2026-05)
────────────────────────────────────────
CI Failure Rate:        12.5% (3/24 PR builds)
Avg Builds per PR:      2.4
PR Lead Time:           avg 18.2h  p50 6.1h  p90 52.0h  (10 PRs)
Review Wait Time:       avg 3.4h (8 PRs)
PR Size Distribution:   XS:4  S:3  M:2  L:1

Daily Build Duration (avg seconds):
  2026-05-02: 74s (6 builds)
  2026-05-03: 81s (4 builds)

DORA (deployment = PR merged into default branch)
────────────────────────────────────────
Deployment Frequency:   10 deploys into main  (2.26/week, 6 deploy days)
Lead Time for Changes:  avg 20.5h  p50 8.0h  p90 60.2h  (10 deploys)
Change Failure Rate:    20.0% (2/10)  reverts=1 hotfixes=1 (label "hotfix")
Recovery Time:          avg 2.8h  p50 2.8h  p90 3.0h  (2 samples)  from reverts=1 incidents=1 (label "incident")
```

#### DORA definitions

Every event is counted in the window its end time falls in.

| Metric | Definition |
|---|---|
| Deployment Frequency | PRs merged into the repo's default branch. Also shown per week and as distinct UTC deploy days. For a window that is still in progress (the default, current month), the per-week rate uses the days elapsed so far. |
| Lead Time for Changes | Earliest commit **author** time in the PR → merge. Author time survives rebases. Only the first 100 commits of a PR are inspected. Negative values (clock skew) count as 0. |
| Change Failure Rate | (revert + hotfix deployments) ÷ deployments. **Revert**: the title starts with the word "revert", followed by a space, `:`, `(`, `"`, `!` or nothing (so `Revert "x"`, `revert: x` and `revert(api): x` count, while `Revert-safe helper` and `revert/cleanup` do not). Reverting a revert re-lands the change, so `Revert "Revert "x""` is not a revert (an odd number of nested `Revert "…"` is). **Hotfix**: carries `hotfix-label`, or the head branch starts with `hotfix/`. A PR that is both counts once. Shows `n/a` when there are no deployments. |
| Recovery Time | Failed deployment recovery time, from two sources: a revert PR whose body says `Reverts owner/repo#N` (GitHub's revert button), measured from #N's merge to the revert's merge; and an issue carrying `incident-label`, measured from opened to closed. Open incidents are not counted. |

Limitations: pushes straight to the default branch (without a PR) are not seen. A hotfix PR is not linked to the deployment it fixed. If the default branch is unknown, the section asks you to run `devpulse repo refresh`.

> PRs synced before the DORA columns existed have no base branch, so they are not counted as deployments until `repo sync` fills them in (the PR fact completion step). On a large history this takes one detail, reviews and commits fetch per PR and may span several syncs if the GitHub rate limit is hit. With `CACHE_ENABLED=true` the detail and reviews come from cached responses, which already contain the new fields.

**Example**

```sh
devpulse metrics MilesChou/devpulse --from 2026-05 --to 2026-06
```

---

### `migrate up`

```
devpulse migrate up
```

Applies all pending database schema migrations. Safe to run multiple times — already-applied migrations are skipped.

**Output**

```
migrations up: ok
```

---

### `migrate down`

```
devpulse migrate down
```

Rolls back the most recently applied migration (one step).

**Output**

```
migrations down: ok
```

---

### `migrate status`

```
devpulse migrate status
```

Prints the list of applied migration versions.

**Output**

```
applied 3 migrations:
  1
  2
  3
```

---

### `worker`

```
devpulse worker [--poll <duration>] [--lease <duration>]
```

Runs the long-running job worker. The worker polls the database for queued jobs (e.g. enrichment jobs enqueued by `sync` / `repo sync`) and processes them. Stop with `Ctrl-C` (`SIGINT`) or `SIGTERM`.

**Flags**

| Flag | Default | Description |
|---|---|---|
| `--poll` | `5s` | Poll interval between empty-queue ticks |
| `--lease` | `60s` | Lease duration before a stuck job is requeued |

**Output**

```
worker started; press Ctrl-C to stop
worker stopped
```

**Example**

```sh
# Run with a faster poll interval during development
devpulse worker --poll 2s
```

---

### `serve`

```
devpulse serve
```

Runs the JSON API (metrics, plus repo management) until `Ctrl-C` (`SIGINT`) or `SIGTERM`. The [desktop dashboard](../desktop/README.md) is its client; anything that speaks HTTP can use it too.

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDR` | `127.0.0.1:8080` | Listen address |
| `DEVPULSE_API_TOKEN` | *(empty)* | Bearer token every `/api/` request must send. **Required** when `HTTP_ADDR` is not a loopback address; `serve` refuses to start otherwise, because the API exposes every tracked repo's data |

In a container, the loopback default is unreachable from outside: set `HTTP_ADDR=0.0.0.0:8080` (and therefore `DEVPULSE_API_TOKEN`) and publish the port.

**Endpoints**

| Method + path | Auth | Returns |
|---|---|---|
| `GET /healthz` | none | `{"status":"ok"}` |
| `GET /api/v1/repos` | bearer | `{"repos":[{id, full_name, owner, name, provider, description, default_branch, disabled}]}` |
| `GET /api/v1/repos/{owner}/{name}` | bearer | One repo, same shape |
| `GET /api/v1/repos/{owner}/{name}/metrics?from=YYYY-MM&to=YYYY-MM` | bearer | The report for the window (below) |
| `GET /api/v1/repos/{owner}/{name}/metrics/monthly?from=YYYY-MM&to=YYYY-MM` | bearer | `{repo, from, to, months:[report, ...]}`, one report per month, oldest first |
| `POST /api/v1/repos` with `{"full_name": "owner/name"}` | bearer | Registers the repo, like `repo add`: `201` `{repo, created: true, metadata_error}`, or `200` when it was already tracked. `metadata_error` is set (and the repo still registered) when GitHub metadata could not be fetched |
| `PATCH /api/v1/repos/{owner}/{name}` with any of `{"pr_start", "incident_label", "hotfix_label"}` | bearer | Updates the settings `repo config set` manages and returns the repo. The same validation applies; one invalid field rejects the whole request |
| `DELETE /api/v1/repos/{owner}/{name}` | bearer | Stops tracking the repo and deletes its synced data, like `repo remove`: `204` |
| `POST /api/v1/repos/{owner}/{name}/sync` | bearer | Starts `repo sync` for it in the background: `202` with the sync status, `409` while another sync runs (one at a time, like `devpulse sync`), `503` when the server has no `GITHUB_TOKEN` |
| `GET /api/v1/sync` | bearer | Background sync status: `{running, started_at, last_repo, last_finished_at, last_error}` |

Repo objects carry the settings too: `pr_start`, `incident_label`, `hotfix_label`. Request bodies must be a single JSON object with known fields only, so a misspelt field (`pr-start`) is rejected with `400` rather than ignored.

#### People

| Method + path | Returns |
|---|---|
| `GET /api/v1/members` | `{"members":[{id, display_name, accounts, team_ids}]}` |
| `POST /api/v1/members` with `{"display_name", "accounts": [...]}` | `201` the member |
| `PUT /api/v1/members/{id}` with the same body | The member; replaces its name and accounts |
| `DELETE /api/v1/members/{id}` | `204`; also removes it from its teams |
| `GET /api/v1/teams` | `{"teams":[{id, name, member_ids}]}` |
| `POST /api/v1/teams`, `PUT /api/v1/teams/{id}` with `{"name", "member_ids": [...]}` | The team |
| `DELETE /api/v1/teams/{id}` | `204`; its members stay |
| `GET /api/v1/excluded-accounts` | `{"accounts": [...]}` |
| `PUT /api/v1/excluded-accounts` with `{"accounts": [...]}` | Replaces the list; returns it normalized |
| `GET /api/v1/repos/{owner}/{name}/metrics/by-member?from=&to=` | `{repo, from, to, rows:[{member_id, name, accounts, report}]}` |

#### Across all repos

These cover every tracked repo that is not disabled, pooling their PRs, builds and reviews, so averages and p50 / p90 are exact across repos rather than averages of per-repo numbers.

| Method + path | Returns |
|---|---|
| `GET /api/v1/metrics?from=&to=` | The report across all repos, same shape as the per-repo one, with `repo: "*"` and `dora: null` (DORA is per repo). Accepts `member` / `team` |
| `GET /api/v1/metrics/monthly?from=&to=` | `{repo: "*", from, to, months:[report, ...]}`; at most 120 months. Accepts `member` / `team` |
| `GET /api/v1/overview/repos?from=&to=` | Comparison rows, one per repo: `{from, to, previous:{from, to}, rows:[{repo, current, previous, monthly:[{month, summary}]}]}` |
| `GET /api/v1/overview/members?from=&to=` | The same for people across all repos: members active in the current or previous period, then active accounts no member claims (`member_id: null`); rows carry `member_id, name, accounts` instead of `repo` |

`previous` is the period of the same number of months right before `from`; `monthly` is the 12 months ending at `to`, for sparklines. A summary has `prs_opened`, `prs_merged`, `lead_time_p50_hours` (the median, so a few PRs left open for weeks do not dominate), `builds_per_pr`, `ci_failure_rate` (0–1), `avg_build_seconds` (per build, so a busy day weighs more), `review_wait_hours` and, for repo rows, `deploys_per_week`. A metric without data in the period is `null`, never 0. Excluded accounts never appear.

Accounts are normalized: lower-cased, with a trailing `[bot]` removed, because GitHub's REST API calls a bot `dependabot[bot]` and its GraphQL API calls the same bot `dependabot`. An account belongs to at most one member, and display and team names are unique; a clash answers `409`.

`metrics` and `metrics/monthly` accept `member=<id>`, `team=<id>` or `account=<login>` (one of them) to cover only that person's, team's or single account's work; `account` lets you look at an account no member claims yet. The report then carries `scope: {kind, id, name, accounts}` (`null` otherwise) and `dora` is `null`, since DORA measures the whole repo. `by-member` lists one row per member active in the window and one per active account no member claims (`member_id: null`, `name` = the account), each with its own report.

The API has one token for reads and writes: whoever holds `DEVPULSE_API_TOKEN` can also add and remove repos. On a loopback address without a token, any local process can. That fits the single-user scope; keep the token private.

`from` / `to` behave exactly like the `metrics` command flags: `from` defaults to the current month (UTC), `to` is exclusive and defaults to `from` + 1 month. `metrics/monthly` accepts at most 120 months per request (it runs one report per month); `metrics` has no width limit. The report is computed by the same code as `devpulse metrics`, so the two never disagree.

**Report**

```json
{
  "repo": "MilesChou/devpulse",
  "from": "2026-05",
  "to": "2026-06",
  "build_failure": { "total": 3, "failed": 2, "rate": 0.6666666666666666 },
  "avg_builds_per_pr": 1.5,
  "pr_lead_time": { "count": 3, "avg_hours": 20, "p50_hours": 20, "p90_hours": 28 },
  "review_wait": { "count": 3, "avg_hours": 2 },
  "pr_size_distribution": [
    { "bucket": "XS", "count": 1 }, { "bucket": "S", "count": 1 },
    { "bucket": "M", "count": 0 }, { "bucket": "L", "count": 1 },
    { "bucket": "XL", "count": 0 }
  ],
  "daily_build_duration": [
    { "day": "2026-05-01", "avg_seconds": 90, "count": 2 }
  ],
  "dora": {
    "default_branch": "main",
    "hotfix_label": "hotfix",
    "incident_label": "incident",
    "deployments": 3,
    "per_week": 2.1,
    "deploy_days": 3,
    "lead_time": { "count": 3, "avg_hours": 22, "p50_hours": 22, "p90_hours": 30 },
    "reverts": 1,
    "hotfixes": 0,
    "change_failure_rate": 0.3333333333333333,
    "recovery": { "count": 2, "avg_hours": 36, "p50_hours": 36, "p90_hours": 61.6 },
    "recovery_from_reverts": 1,
    "recovery_from_incidents": 1
  }
}
```

`pr_size_distribution` always lists the five buckets in ascending size order, plus an `unknown` entry only when some PRs have no bucket. `dora` follows the [DORA definitions](#dora-definitions) and is `null` while the repo's default branch is unknown (run `repo refresh`); inside it, `change_failure_rate` is `null` when there were no deployments. For a window still in progress, `per_week` is measured up to now, as in the `metrics` command. The canonical examples are the golden files in [`internal/http/testdata/`](../internal/http/testdata/); the dashboard's tests decode the same files.

**Errors** are JSON `{"error": "..."}` with status `400` (bad `from` / `to`, invalid body or setting), `401` (missing or wrong token), `404` (repo not tracked, unknown path), `409` (a sync is already running), `503` (sync unavailable), or `500` (details are logged server-side, not returned).

**Example**

```sh
DEVPULSE_API_TOKEN=change-me devpulse serve
curl -H "Authorization: Bearer change-me" \
  "http://127.0.0.1:8080/api/v1/repos/MilesChou/devpulse/metrics?from=2026-05"
```

---

## Typical Workflow

```sh
# 1. Apply schema migrations
devpulse migrate up

# 2. Register the target repository
devpulse repo add MilesChou/devpulse

# 2'. (Optional) Skip early no-CI history by setting the PR floor.
#     Without this the first sync walks PR #1 onward.
devpulse repo config set MilesChou/devpulse pr-start 500

# 3. Back-fill pull requests (with enrichment) and CI builds for that
#    one repo
devpulse repo sync MilesChou/devpulse

# 3'. ...or, once you have multiple repos registered, sync them all in
#     one shot — this is also the command to put on a cron / CI schedule.
devpulse sync

# 4. (Optional) Refresh a single PR
devpulse pr sync MilesChou/devpulse 42

# 5. (Optional) Run the background worker for async enrichment jobs
devpulse worker

# 6. (Optional) Serve the metrics API for the desktop dashboard
DEVPULSE_API_TOKEN=change-me devpulse serve
```

## Development Shortcuts (Makefile)

The `Makefile` at the repository root provides convenience targets. Run `make help` to list them.

| Target | Description |
|---|---|
| `make build` | Compile the binary to `./bin/devpulse` |
| `make run ARGS="..."` | Build, load `.env`, then run `./bin/devpulse <ARGS>` |
| `make test` | Run unit tests |
| `make test-race` | Run unit tests with `-race` |
| `make test-integration` | Run integration tests (requires Docker) |
| `make lint` | Run `go vet` + `gofmt` check |
| `make tidy` | Run `go mod tidy` |
| `make clean` | Remove `./bin/` |
| `make desktop` | Build the Rust desktop dashboard (release) |
| `make desktop-run` | Run the desktop dashboard (debug build) |
| `make desktop-test` | Run the desktop dashboard tests |
| `make desktop-lint` | `cargo fmt --check` + `clippy` for the dashboard |

**Example**

```sh
make run ARGS="repo sync MilesChou/devpulse"
```
