# DevPulse

Engineering-efficiency observability for CI and PR workflows: pulls data
from GitHub and CI providers, computes team-level metrics (CI failure
rate, PR review latency, build duration, PR re-push count, and the four
DORA metrics), and persists them to a relational database for downstream
analysis.

Distributed as a single Go binary. `devpulse serve` exposes the metrics
as a read-only JSON API, and an optional Rust desktop dashboard
([`desktop/`](desktop/README.md)) charts them.

> 正體中文：[README.zh-TW.md](README.zh-TW.md)

## Scope

- **Is**: a CLI tool plus a relational data layer, with a read-only
  HTTP API and a desktop dashboard on top.
- **Is not**: a SaaS, a multi-tenant platform, or a realtime webhook
  service.
- **Designed for**: single-host, single-user, ~100–1000 builds per repo
  per month.

## Install

### Requirements

- Go **1.26+** (only for building from source)
- A supported database: PostgreSQL, MySQL, or SQLite (including in-memory)
- A GitHub personal access token; a Travis CI token if Travis is used
- Rust **1.95+**, only for building the desktop dashboard

### Build from source

```bash
git clone https://github.com/MilesChou/devpulse.git
cd devpulse
make build
./bin/devpulse --help
```

Or install directly:

```bash
go install github.com/mileschou/devpulse/cmd/devpulse@latest
```

## Configuration

Copy the example file and fill in the secrets:

```bash
cp .env.example .env
```

`DEVPULSE_DSN` accepts the following forms:

```
postgres://user:pass@host:5432/db?sslmode=disable
mysql://user:pass@host:3306/db?parseTime=true
sqlite://./devpulse.db?_fk=true
memory                              # in-memory SQLite, auto-migrates on startup
```

The `memory` form requires no external services and is convenient for
testing or one-off CLI invocations.

## Quick start

```bash
# Apply migrations (skipped automatically when DEVPULSE_DSN=memory).
devpulse migrate up

# Register a repository.
devpulse repo add MilesChou/devpulse

# Sync one repo: PRs (with reviews and enrichment) first, then CI
# builds from every provider (GitHub Actions always; Travis CI when
# TRAVIS_TOKEN is set). The first run touches the full histories and
# consumes a significant share of the REST/GraphQL quota; subsequent
# runs are incremental (per-provider watermarks, upsert dedupe, and
# author back-fill skips populated rows).
devpulse repo sync MilesChou/devpulse

# Or sync every tracked repo in one go (sequential; disabled repos are
# skipped; per-repo failures are aggregated into a final summary so one
# bad repo does not block the rest).
devpulse sync

# Re-sync a single PR (re-fetch detail and reviews).
devpulse pr sync MilesChou/devpulse 42

# Show the month's engineering-efficiency metrics.
devpulse metrics MilesChou/devpulse --from 2026-05

# Process enqueued jobs (long-running).
devpulse worker

# Serve the metrics as JSON for the desktop dashboard (long-running).
DEVPULSE_API_TOKEN=change-me devpulse serve
```

An optional disk-backed HTTP response cache (`CACHE_ENABLED=true`) can
replay previously fetched API responses — useful for rebuilding the
database without burning API quota. See
[docs/commands.md](docs/commands.md) for the cache variables and the
`CACHE_TTL=0` replay-mode caveat.

## Local exploration with Metabase

For interactive browsing of the synced build, PR, and review data,
an optional Metabase overlay ships with the repository:

```bash
docker compose \
    -f docker-compose.yml \
    -f docker-compose.postgres.yml \
    -f docker-compose.metabase.yml up -d --wait
```

The `metabase-init` sidecar auto-bootstraps:

- Admin account: `admin@devpulse.local` / `changeme1!` (local dev only)
- Data source: the DevPulse PostgreSQL, pre-registered as **DevPulse**

So `up -d --wait` is the entire setup — no first-run wizard, no manual
data-source entry. Then browse to [http://localhost:3000](http://localhost:3000)
and log in.

If the init container exits non-zero, look at its log to diagnose:

```bash
docker compose -f docker-compose.metabase.yml logs metabase-init
```

To start over from a clean Metabase: `docker compose down`, then
`docker volume rm devpulse_metabase-data`, then `up -d --wait` again.
The Postgres data (your synced PRs, builds, reviews) is in a separate
volume and is not affected.

## DORA metrics

`devpulse metrics` includes the four DORA metrics. They are derived from
data the regular sync already collects:

| Metric | How DevPulse measures it |
|---|---|
| Deployment Frequency | PRs merged into the default branch |
| Lead Time for Changes | Earliest commit author time in the PR → merge |
| Change Failure Rate | (revert + hotfix PRs) ÷ deployments |
| Recovery Time | Reverted PR merge → revert merge, and incident issue opened → closed |

Hotfix PRs and incident issues are recognised by label. Set them per
repo with `devpulse repo config set <repo> hotfix-label <label>` and
`incident-label <label>` (defaults: `hotfix`, `incident`). Full
definitions and limitations are in
[docs/commands.md](docs/commands.md#dora-definitions).

## Desktop dashboard

[`desktop/`](desktop/README.md) is a native dashboard written in Rust
(egui) that reads the `devpulse serve` API: KPI cards for CI failure
rate, builds per PR, PR lead time and review wait (each with its ideal
value and month-over-month change), the four DORA metrics, PR size
distribution, daily build duration, and 12-month trends.

![Desktop dashboard](docs/images/desktop-dashboard.jpg)

```bash
DEVPULSE_API_TOKEN=change-me devpulse serve   # on the host with the DB
make desktop-run                              # then enter URL + token in Settings
```

The dashboard only holds the DevPulse API token (in the OS keychain);
GitHub and CI tokens stay on the server. See
[`serve`](docs/commands.md#serve) for the endpoints and the JSON shape.

## Commands

DevPulse groups commands by resource (`repo`, `pr`) with verbs underneath,
in the style of `gh` and `jira-cli`. `sync` is the one top-level verb —
it fans out across every tracked repo and is the natural entry point for
cron / CI.

| Command | Purpose |
|---|---|
| `devpulse sync` | Sync every tracked repo (sequential; skips disabled; aggregates failures) |
| `devpulse repo add <owner/name>` | Register a repository |
| `devpulse repo sync <owner/name>` | Sync one repo: refresh open PRs and PRs changed upstream, new PRs (with enrichment), missing DORA facts on stored PRs, CI builds (linked to their PR), incident issues |
| `devpulse pr sync <owner/name> <number>` | Re-sync a single PR (detail + reviews) |
| `devpulse metrics <owner/name>` | Print engineering-efficiency and DORA metrics for a month window |
| `devpulse migrate {up,down,status}` | Schema migration |
| `devpulse worker` | Run the DB-backed job worker |
| `devpulse serve` | Serve repos and metrics as a read-only JSON API |

## Development

```bash
make all       # gofmt + go vet + go test + build (used by the pre-commit hook)
make build     # Build the binary into ./bin/devpulse
make test      # Run unit tests
make test-race # Run unit tests with the race detector
make lint      # gofmt + go vet
make tidy      # go mod tidy

make desktop-test  # cargo test for the desktop dashboard
make desktop-lint  # cargo fmt --check + clippy
make desktop       # release build of the dashboard
```

By default `make test` runs against in-memory SQLite. To run the same
test suite against a real PostgreSQL or MySQL instance, point
`DEVPULSE_DSN` at it and serialize the tests (the suite resets migrations
between cases, so parallel runs would race):

```bash
DEVPULSE_DSN='postgres://devpulse:devpulse@localhost:5432/devpulse?sslmode=disable' \
  go test -p 1 -race -count=1 ./...
```

A pair of Docker Compose overlays is provided to spin up a local backend.
The base file is intentionally empty; pick one or both overlays:

```bash
docker compose -f docker-compose.yml -f docker-compose.postgres.yml up -d
docker compose -f docker-compose.yml -f docker-compose.mysql.yml    up -d
```

CI runs the SQLite, PostgreSQL, and MySQL matrix automatically — see
[`.github/workflows/ci.yml`](.github/workflows/ci.yml).

### Tracing

OpenTelemetry tracing is optional. Set `OTEL_EXPORTER_OTLP_ENDPOINT` to a
collector address (e.g. a local Jaeger at `localhost:4318`) to ship
spans; leave it empty and the provider is a no-op.

## Stack

- Go 1.26
- `database/sql` with three drivers: `jackc/pgx/v5/stdlib`,
  `go-sql-driver/mysql`, `modernc.org/sqlite`
- [`spf13/cobra`](https://github.com/spf13/cobra) for the CLI
- [`cli/go-gh`](https://github.com/cli/go-gh) for the GitHub HTTP client
  (default headers + ASCII sanitizer; does not retry)
- [`hashicorp/go-retryablehttp`](https://github.com/hashicorp/go-retryablehttp)
  for the Travis HTTP client (and any other generic outbound HTTP)
- OpenTelemetry SDK for tracing
- A small in-tree DB-backed job queue
- `net/http` (Go 1.22+ routing patterns) for the JSON API
- Desktop dashboard: Rust with [`eframe`/`egui`](https://github.com/emilk/egui),
  [`egui_plot`](https://github.com/emilk/egui_plot),
  [`ureq`](https://github.com/algesten/ureq) and
  [`keyring`](https://github.com/open-source-cooperative/keyring-rs)

## License

MIT — see [LICENSE](LICENSE).
