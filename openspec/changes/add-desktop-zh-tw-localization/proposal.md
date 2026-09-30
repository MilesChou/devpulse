## Why

The desktop dashboard only speaks English: every label, KPI card, chart title and error message is a hardcoded English string in `desktop/src/`, and egui's bundled fonts have no CJK glyphs, so even a single Chinese string would render as boxes. The project already ships Traditional Chinese (Taiwan) docs for every README; the dashboard is the one user-facing surface without it.

## What Changes

- Load a CJK-capable font at startup so Traditional Chinese text (UI strings, and repo names / descriptions coming from the API) renders instead of showing boxes.
- Move every user-facing string in `desktop/src/` (top bar, settings panel, repo list, KPI and DORA cards, chart titles and legends, notices, `ApiError` messages) into a localization layer with two languages: English and Traditional Chinese (Taiwan).
- Pick the language on first launch from the OS locale (Traditional Chinese locales → zh-TW, anything else → English), let the user switch it in Settings, and persist the choice in `desktop.json`.
- Add a `DEVPULSE_DESKTOP_LANG` environment variable that sets the language for one run without touching the settings file, matching the existing `DEVPULSE_SERVER_URL` / `DEVPULSE_API_TOKEN` overrides.
- Left untranslated on purpose: data values (repo names, months, numbers), command names in hints (`devpulse repo refresh …`), metric abbreviations that are industry terms (CI, PR, DORA, p50 / p90), and server-provided error detail text.
- Trend charts follow the selected window when it is longer than twelve months, instead of always showing the last twelve months. Found while using the dashboard on a repo with years of history: a 2020–2026 window showed full KPI cards over empty trend charts.
- Raise the server's `metrics/monthly` limit from 36 to 120 months (`MaxMonths` in `internal/metrics`), so a multi-year window can be charted in one request; the dashboard caps its trend request at the same 120 months.
- Update `desktop/README.md` and `desktop/README.zh-TW.md` (language setting, env var, font requirement, troubleshooting row), plus a zh-TW screenshot.

## Capabilities

### New Capabilities

- `desktop-localization`: which languages the dashboard offers, how the language is chosen, persisted and overridden, what text is and is not translated, and that Chinese text renders with a CJK font.

### Modified Capabilities

- `desktop-dashboard`: the "Token storage" scenario says the settings file holds only the server URL and the last selected repo; it now also holds the chosen UI language. The "Twelve-month trend" scenario gains companions: windows longer than twelve months are charted in full, up to the server's 120-month limit.
- `http-api`: the "Oversized range is rejected" scenario moves the monthly trend limit from 36 to 120 months.

## Impact

- **Code**: `desktop/src/app.rs` (≈35 UI strings, Settings panel gains a language selector), `desktop/src/kpi.rs` (card titles, details, targets, delta suffixes; unit tests assert English text), `desktop/src/api.rs` (`ApiError` display text), `desktop/src/settings.rs` (new `language` field), `desktop/src/main.rs` (font setup, env override), plus a new localization module.
- **Dependencies**: likely one crate for OS locale detection and one for locating system fonts, or an embedded font file; the choice is made in design.md. The only Go change is the `MaxMonths` constant (36 → 120) with its tests and `docs/commands*.md`; the API shape is unchanged.
- **Binary size / platforms**: depends on the font strategy (embedding a CJK font adds several MB; system lookup adds per-OS behaviour on macOS, Windows and Linux).
- **Docs**: `desktop/README.md`, `desktop/README.zh-TW.md`, `docs/images/` (new screenshot). The root READMEs only link to the desktop README and need no change unless design.md decides otherwise.
- **CI**: the existing `Desktop (Rust)` job (fmt, clippy, test) covers the new code; tests must pass regardless of the CI machine's locale.
