# DevPulse desktop dashboard

A native dashboard for the metrics DevPulse collects, written in Rust
with [egui](https://github.com/emilk/egui). It is a read-only client of
the JSON API that `devpulse serve` exposes: the Go service does all the
data collection (GitHub, CI providers, the database), and this app only
needs the server URL and its API token.

> 正體中文：[README.zh-TW.md](README.zh-TW.md)

![Dashboard](../docs/images/desktop-dashboard.jpg)

It shows, per repo and month window:

- **KPI cards** for the goals in the project brief: CI failure rate
  (ideal 0%), builds per PR as the re-push proxy (ideal 1), PR lead
  time created → merged (ideal 24h), and review wait. Single-month
  windows also show the change against the previous month.
- **PR size distribution** (ideal: mostly XS / S) and **daily build
  duration**.
- **DORA cards**: deployment frequency, lead time for changes, change
  failure rate, and recovery time, with month-over-month change. The
  project goals set no DORA targets, so the cards show which direction
  is better instead of an ideal value. When the server does not know
  the repo's default branch yet, the panel says to run
  `devpulse repo refresh`.
- **12-month trends** of CI failure rate, PR lead time (avg / p50 /
  p90), deployments per week, and change failure rate, ending at the
  selected window.

## Run

Start the API on the machine that holds the DevPulse database (see
[`serve`](../docs/commands.md#serve)):

```bash
DEVPULSE_API_TOKEN=change-me devpulse serve
```

Then build and run the dashboard (Rust 1.95+):

```bash
make desktop-run
```

Open **Settings**, enter the server URL (default
`http://127.0.0.1:8080`) and the API token, and press **Save &
connect**. **Test** checks both that the server is up and that the
token is accepted.

A server on another host must listen on a non-loopback `HTTP_ADDR`,
which requires `DEVPULSE_API_TOKEN`. The API is plain HTTP, so put it
behind a TLS reverse proxy (or an SSH tunnel) when it leaves a trusted
network.

## Where settings live

| What | Where |
|---|---|
| API token | The OS keychain (macOS Keychain, Windows Credential Manager, Secret Service on Linux), one entry per server URL |
| Server URL, last selected repo | `desktop.json` in the OS config directory (`~/Library/Application Support/devpulse/` on macOS) |

GitHub and CI tokens never reach the dashboard; they stay on the
server.

Environment variables for scripted launches and development. They
apply to that run only and are never written to the settings file or
the keychain:

| Variable | Effect |
|---|---|
| `DEVPULSE_SERVER_URL` | Server URL for this run |
| `DEVPULSE_API_TOKEN` | API token for this run (skips the keychain) |
| `DEVPULSE_DESKTOP_CONFIG` | Path of the settings file |

## Develop

```bash
make desktop-test   # cargo test
make desktop-lint   # cargo fmt --check + clippy -D warnings
make desktop        # release build: desktop/target/release/devpulse-desktop
```

The tests decode the Go API's golden files in
[`internal/http/testdata/`](../internal/http/testdata/), so a change to
the API's JSON shape fails here as well as in the Go tests. After an
intended change, regenerate them with
`go test ./internal/http/ -run Golden -update` and update the Rust
types in `src/api.rs`.

| File | Role |
|---|---|
| `src/api.rs` | HTTP client and the API types |
| `src/state.rs` | Dashboard state and how API results update it |
| `src/kpi.rs` | KPI card text and month-over-month deltas |
| `src/month.rs` | `YYYY-MM` month arithmetic |
| `src/settings.rs` | Settings file and keychain access |
| `src/app.rs` | egui rendering; API calls run on worker threads |
