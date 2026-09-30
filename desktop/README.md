# DevPulse desktop dashboard

A native dashboard for the metrics DevPulse collects, written in Rust
with [egui](https://github.com/emilk/egui). It is a client of the JSON
API that `devpulse serve` exposes: the Go service does all the data
collection (GitHub, CI providers, the database), and this app only
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
- **Trends** of CI failure rate, PR lead time (avg / p50 / p90),
  deployments per week, and change failure rate: the 12 months ending at
  the selected window, or the whole window when it is longer (at most
  the last 120 months, the server's limit for one monthly trend).

The app opens on the **Overview**: a repo table and a member table for
the selected period, to spot which repo got slower or fails more, and
how people compare across all repos. Each metric shows its change
against the previous period of the same length; a change for the worse
beyond a noise floor (2 percentage points for the CI failure rate, 10 %
otherwise) is highlighted. Click a column header to sort, worst first
(again to reverse); the last column is a 12-month sparkline of the
sorted metric. Click a repo to open its dashboard, or a member to open
their dashboard across all repos.

**All repos** at the top of the repo list is that person-first view:
with a member picked in **Show**, the cards, charts and trends cover
their work in every repo. DORA is measured per repo, so its panel asks
for a single repo.

The **Show** picker next to the repo name limits everything on the
dashboard to one team or one member (DORA stays whole-repo only). Below
the DORA panel, **By member** lists everyone active in the window with
their PRs, lead time, builds per PR, CI failures and review wait;
accounts no member claims yet are listed in italics with a **Map…**
button.

The **People** page maps GitHub accounts to members (one person, any
number of accounts), groups members into teams, and edits the excluded
accounts: bots whose PRs, builds and reviews count in no metric except
DORA. It starts with `dependabot`, `github-actions` and
`copilot-pull-request-reviewer`.

The **Repos** page (top bar) manages what the server tracks: add a repo
by `owner/name`, edit its settings (PR start, incident and hotfix
labels), start a sync, or remove it together with its synced data
(asks for confirmation first). Syncing from the dashboard needs
`GITHUB_TOKEN` on the server; without it the page says so and
`devpulse sync` on the server remains the way to sync. One sync runs
at a time, and the page shows its progress.

## Build

The dashboard is a standalone Rust crate. Building it needs neither Go
nor a database; those are only needed on the machine that runs
`devpulse serve`.

### 1. Install Rust

The crate requires Rust **1.95 or newer** (`rust-version` in
`Cargo.toml`). Install it with [rustup](https://rustup.rs):

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

On Windows, download and run `rustup-init.exe` from
[rustup.rs](https://rustup.rs) instead. If Rust is already installed,
update it and check the version:

```bash
rustup update stable
rustc --version
```

### 2. Install the platform prerequisites

| Platform | What to install |
|---|---|
| macOS | Xcode Command Line Tools: `xcode-select --install` |
| Windows | [Visual Studio Build Tools](https://visualstudio.microsoft.com/visual-cpp-build-tools/) with the **Desktop development with C++** workload (rustup's default `msvc` toolchain links with it) |
| Debian / Ubuntu | The packages below |

```bash
sudo apt-get install build-essential pkg-config \
  libxcb-render0-dev libxcb-shape0-dev libxcb-xfixes0-dev libxkbcommon-dev
```

The Linux package list is the one [eframe](https://github.com/emilk/egui/tree/main/crates/eframe)
documents, minus `libssl-dev`: this crate uses rustls for HTTPS and a
pure-Rust D-Bus client for the keychain, so it links neither OpenSSL
nor libdbus. CI builds on `ubuntu-latest` with exactly these packages.
On other distributions, install the equivalent xcb and xkbcommon
development packages.

To show Chinese text, the machine that *runs* the dashboard also needs a
Traditional Chinese font; see [Language](#language).

### 3. Build

From the repository root:

```bash
cd desktop
cargo build --release --locked
```

With `make` available, `make desktop` from the repository root does the
same. `--locked` builds with the dependency versions in the committed
`Cargo.lock`, which are the ones CI tests.

The first build downloads and compiles every dependency (about a minute
on an Apple M-series laptop); later builds are incremental. The result
is a single executable with no runtime files beside it:

| Platform | Executable |
|---|---|
| macOS, Linux | `desktop/target/release/devpulse-desktop` |
| Windows | `desktop\target\release\devpulse-desktop.exe` |

Copy it anywhere and run it. There is no installer or macOS `.app`
bundle yet, so on macOS you start it from a terminal like any other
binary.

For a quick debug build that also starts the app, run `cargo run` in
`desktop/` (or `make desktop-run` from the repository root).

## Run

The dashboard needs a running DevPulse API. On the machine that holds
the DevPulse database (see [`serve`](../docs/commands.md#serve)):

```bash
DEVPULSE_API_TOKEN=change-me devpulse serve
```

Then start the dashboard, open **Settings** (**設定** in the Chinese UI), enter the server URL
(default `http://127.0.0.1:8080`) and the API token, and press **Save &
connect**. **Test** checks both that the server is up and that the
token is accepted.

### Server on another machine

`devpulse serve` listens on `127.0.0.1:8080` by default, which other
machines cannot reach. On the server, listen on all interfaces; this
requires the token, and `serve` refuses to start without it:

```bash
HTTP_ADDR=0.0.0.0:8080 DEVPULSE_API_TOKEN=change-me devpulse serve
```

In the dashboard, use the server's address, e.g.
`http://192.168.1.10:8080`, and allow port 8080 through the server's
firewall. The API is plain HTTP, so outside a trusted network put it
behind a TLS reverse proxy, or keep the server on loopback and use an
SSH tunnel:

```bash
ssh -N -L 8080:127.0.0.1:8080 user@server   # then use http://127.0.0.1:8080
```

### Try it end to end on one machine

With Go installed as well (see the [main README](../README.md#install)),
from the repository root:

```bash
make build
export DEVPULSE_DSN=sqlite://./devpulse.db GITHUB_TOKEN=<your token>
./bin/devpulse migrate up
./bin/devpulse repo add <owner/name>
./bin/devpulse repo sync <owner/name>
DEVPULSE_API_TOKEN=change-me ./bin/devpulse serve
```

Then, in a second terminal, run `make desktop-run` and connect to
`http://127.0.0.1:8080` with the token `change-me`.

### Troubleshooting

| Symptom | Fix |
|---|---|
| Cargo refuses to build because the package requires a newer rustc | `rustup update stable` |
| Linker errors mentioning `xcb` or `xkbcommon` (Linux) | Install the packages in step 2 |
| `link.exe` not found (Windows) | Install the Visual Studio Build Tools in step 2 |
| "Cannot read the keychain" or "Could not save the token" | No OS keychain is available, typically Linux without GNOME Keyring or KWallet running. The token then lasts only for the session; set `DEVPULSE_API_TOKEN` when launching to skip the keychain |
| "cannot reach server" | Check the URL, that `devpulse serve` is running, and the server's `HTTP_ADDR` and firewall |
| "API token was rejected (401)" | The token differs from the server's `DEVPULSE_API_TOKEN` |
| Chinese text shows as boxes, and Settings says no Chinese font was found | Install one of the fonts listed under [Language](#language), then restart the dashboard |

## Language

The dashboard is available in English and Traditional Chinese (Taiwan).
On first launch it follows the OS language: `zh-TW`, `zh-HK`, `zh-MO`
and other Traditional Chinese (`zh-Hant`) locales get Chinese,
everything else, including Simplified Chinese, gets English. Change it
under **Settings → Language**; the switch is immediate and remembered.

Repo names, months, numbers, the abbreviations CI, PR, DORA, p50 and
p90, CLI commands in hints, and error details returned by the server
are never translated.

egui's built-in fonts have no Chinese glyphs, so at startup the
dashboard loads the first of these system fonts it finds and uses it
for any character the built-in fonts lack, in either language:

| Platform | Fonts searched, in order |
|---|---|
| macOS | PingFang TC, Heiti TC, Hiragino Sans CNS (all ship with macOS) |
| Windows | Microsoft JhengHei, Microsoft JhengHei UI (ship with Windows) |
| Linux | Noto Sans CJK TC, Noto Sans TC, Source Han Sans TC, WenQuanYi Zen Hei (`sudo apt-get install fonts-noto-cjk` on Debian / Ubuntu) |

If none is found, the dashboard still starts and **Settings** says so.
Font lookup has been verified on macOS only so far.

## Where settings live

| What | Where |
|---|---|
| API token | The OS keychain (macOS Keychain, Windows Credential Manager, Secret Service on Linux), one entry per server URL |
| Server URL, last selected repo, UI language | `devpulse/desktop.json` in the OS config directory: `~/Library/Application Support/` on macOS, `$XDG_CONFIG_HOME` or `~/.config/` on Linux, `%APPDATA%` on Windows |

GitHub and CI tokens never reach the dashboard; they stay on the
server.

Environment variables for scripted launches and development. They
apply to that run only and are never written to the settings file or
the keychain:

| Variable | Effect |
|---|---|
| `DEVPULSE_SERVER_URL` | Server URL for this run |
| `DEVPULSE_API_TOKEN` | API token for this run (skips the keychain) |
| `DEVPULSE_DESKTOP_LANG` | UI language for this run: `en` or `zh-TW` (other values are ignored) |
| `DEVPULSE_DESKTOP_CONFIG` | Path of the settings file |

## Develop

```bash
make desktop-test   # cargo test
make desktop-lint   # cargo fmt --check + clippy -D warnings
make desktop        # cargo build --release
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
| `src/i18n.rs` | UI languages and every UI string, in English and Traditional Chinese |
| `src/fonts.rs` | Finds and loads a system font for Chinese text |
| `src/month.rs` | `YYYY-MM` month arithmetic |
| `src/settings.rs` | Settings file and keychain access |
| `src/app.rs` | egui rendering; API calls run on worker threads |
