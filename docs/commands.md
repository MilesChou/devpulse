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
| `TRAVIS_TOKEN` | Travis CI API token (required by `repo sync`) |

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

### `repo sync`

```
devpulse repo sync <owner/name>
```

Syncs the repository in two steps, in order:

1. **Pull requests** — fetches all PRs from GitHub (including reviews and commit details), upserts them, and runs enrichment.
2. **CI builds** — fetches all CI build records from Travis CI and upserts them.

The PR step runs first; if it fails, the build step is skipped and the command exits non-zero. Both `GITHUB_TOKEN` and `TRAVIS_TOKEN` are required.

> The first run is the expensive one: PR sync pages through the full PR history (a meaningful share of the GitHub REST and GraphQL quota), and build sync walks the full Travis history (capped at 100 pages × 100 builds). Subsequent runs are incremental — upserts dedupe writes, author back-fill only touches commit SHAs whose author is still NULL, and PR pages are upserted/enriched before moving on so a partial run still records progress.

**Arguments**

| Argument | Description |
|---|---|
| `owner/name` | GitHub repository slug |

**Output**

```
Synced MilesChou/devpulse PRs: written=7
Synced MilesChou/devpulse builds: written=42
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

Runs the long-running job worker. The worker polls the database for queued jobs (e.g. enrichment jobs enqueued by `repo sync`) and processes them. Stop with `Ctrl-C` (`SIGINT`) or `SIGTERM`.

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

# 3. Back-fill pull requests (with enrichment) and CI builds
devpulse repo sync MilesChou/devpulse

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
