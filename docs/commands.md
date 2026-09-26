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

Syncs the repository in four steps, in order:

1. **PR refresh** — re-fetches every stored PR that GitHub reports as updated since the previous refresh, plus every PR stored as open, so a merge, close, reopen, retitle or relabel after its first sync is recorded (merges are DORA deployments). The high-water mark is the `updated_at` of the most recently updated PR seen, taken from GitHub's own listing, so a cached listing can never skip updates. A PR whose detail comes back older than the listing (a cached copy) is not written. A PR that fails to refresh is logged, and the mark is not advanced so the next sync lists it again. Open PRs are re-fetched every time so that a stale cached copy, for example one written during a rebuild with `CACHE_ENABLED=true`, heals once its cache entry expires.
2. **Pull requests** — fetches all new PRs from GitHub (detail, reviews, and — for merged PRs — the earliest commit time), upserts them, and runs enrichment.
3. **CI builds** — fetches build records from every registered CI provider (GitHub Actions always; Travis CI when `TRAVIS_TOKEN` is set) and upserts them. GitHub Actions leaves a run's PR list empty in practice, so each PR-triggered build is then linked to the stored PR whose head branch matches the build's branch and that was open when the build started. A build that matches two PRs (for example, two fork PRs from branches named `main` open at once) stays unlinked.
4. **Incidents** — mirrors every issue carrying the repo's `incident-label` (open and closed; pull requests excluded). A failure prints a warning and does not fail the command.

If step 1 or 2 fails, later steps are skipped and the command exits non-zero. `GITHUB_TOKEN` is required; `TRAVIS_TOKEN` is optional.

> The first run is the expensive one: PR sync walks PR numbers ascending from `pr_sync_start_number` (default 1) up to the upstream max, fetching detail + reviews per PR; build sync walks each provider's full history with no page cap (cold-start path triggered when that provider has no rows yet). Subsequent runs are incremental — PRs resume from `MAX(number) + 1`, and each CI provider resumes from its **own** `MAX(started_at) - 6h` watermark (per-provider cursors keep a lagging or newly added provider from inheriting another's progress; the 6-hour overlap absorbs retry builds and runs that were still executing at the previous sync — 6h is the GitHub Actions per-job hard timeout — while the `(repo_id, ci_provider, external_id)` unique dedupes anything already on file). Author back-fill only touches commit SHAs whose author is still NULL.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |

**Output**

```
Refreshed MilesChou/devpulse pull requests: 3
Synced MilesChou/devpulse pull requests: written=7
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

> PRs synced before the DORA columns existed have no base branch, so they are not counted as deployments. Rebuild the database, or start with a fresh one, to fill them in. With `CACHE_ENABLED=true` the rebuild replays cached PR responses, which already contain the new fields.

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

**Placeholder — not implemented in v1.** Prints a notice and exits. The HTTP API surface is planned for a future release.

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

**Example**

```sh
make run ARGS="repo sync MilesChou/devpulse"
```
