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
| `TRAVIS_TOKEN` | Travis CI API token (required only for `build fetch`) |

## Command Reference

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

### `build fetch`

```
devpulse build fetch <owner/name>
```

Fetches all CI build records from Travis CI for the given repository, then writes them to the store. Requires `TRAVIS_TOKEN`.

> The first run walks the full Travis history (capped at 100 pages × 100 builds). Subsequent runs are incremental — upserts dedupe writes and only commit SHAs with a NULL author are sent through the GitHub bulk-author lookup.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |

**Output**

```
Fetched MilesChou/devpulse builds: written=42
```

**Example**

```sh
devpulse build fetch MilesChou/devpulse
```

---

### `pr fetch`

```
devpulse pr fetch <owner/name>
```

Fetches all pull requests (including reviews and commit details) from GitHub for the given repository, then writes them to the store and runs enrichment automatically.

> The first run pages through the full PR history; for repos with thousands of PRs this consumes a meaningful share of the REST and GraphQL quota. Each page is upserted and enriched before moving on, so a partial run still records progress.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |

**Output**

```
Fetched MilesChou/devpulse PRs: written=7
```

**Example**

```sh
devpulse pr fetch MilesChou/devpulse
```

---

### `pr enrich`

```
devpulse pr enrich <owner/name> <number>
```

Re-fetches detail and review data for a single pull request that is already in the store, then writes the enrichment patch. Use this to refresh a specific PR without re-fetching the entire month.

The PR must already exist in the store. If it does not, run `devpulse pr fetch` first.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |
| `number` | Pull request number, e.g. `42` |

**Output**

```
Enriched MilesChou/devpulse#42
```

**Example**

```sh
devpulse pr enrich MilesChou/devpulse 42
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

Runs the long-running job worker. The worker polls the database for queued jobs (e.g. enrichment jobs enqueued by `pr fetch`) and processes them. Stop with `Ctrl-C` (`SIGINT`) or `SIGTERM`.

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

# 3. Back-fill CI builds and pull requests
devpulse build fetch MilesChou/devpulse
devpulse pr fetch MilesChou/devpulse

# 4. (Optional) Refresh a single PR
devpulse pr enrich MilesChou/devpulse 42

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
make run ARGS="pr fetch MilesChou/devpulse"
```
