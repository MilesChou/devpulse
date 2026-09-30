## Context

The desktop dashboard (`desktop/`, eframe/egui 0.36) is English-only. User-facing text is spread across:

- `src/app.rs`: ≈35 literals (top bar, settings panel, repo list, notices, chart titles, legends, hover text).
- `src/kpi.rs`: KPI / DORA card titles, detail lines, targets ("ideal 24h", "lower is better") and delta suffixes; its unit tests assert on the English output.
- `src/api.rs`: `ApiError`'s `Display` ("API token was rejected (401)", "cannot reach server: …"), which `app.rs` shows as-is.
- `src/state.rs`: only format strings with no words (`"{} ~ {}"`).

egui's bundled fonts have no CJK glyphs. `epaint 0.36.2` loads fonts through `FontData { font, index, tweak }`, so a face inside a `.ttc` collection can be selected by index (`epaint-0.36.2/src/text/fonts.rs:109-136`).

On this development Mac, `/System/Library/Fonts/` contains `STHeiti Medium.ttc` (≈55 MB), `STHeiti Light.ttc` and `Hiragino Sans GB.ttc`; PingFang lives under `/System/Library/AssetsV2/com_apple_MobileAsset_Font8/<hash>.asset/`, a path that is not stable across machines. Font files on Windows and Linux have not been checked yet.

## Goals / Non-Goals

**Goals:**

- English and Traditional Chinese (Taiwan) for all text the dashboard itself produces.
- A missing translation is a compile error, not a runtime fallback.
- Chinese renders in both UI languages (API data can contain Chinese under the English UI).
- Language follows the OS locale on first launch, is switchable live in Settings, persisted in `desktop.json`, and overridable per run with `DEVPULSE_DESKTOP_LANG`.

**Non-Goals:**

- Simplified Chinese or any other language. The design must make a third language cheap to add, but none is added now.
- Translating server responses. The Go service and the HTTP API stay English; error *detail* text from the server is shown as received.
- Localized number or date formats. Numbers, `YYYY-MM` months and `%` stay as they are in both languages.
- Localizing the Go CLI.

## Decisions

### 1. Hand-written typed string table instead of an i18n crate

A new `src/i18n.rs` holds:

- `enum Lang { En, ZhTw }` with serde names `"en"` / `"zh-TW"`.
- `struct Texts` whose fields are either `&'static str` or `fn(..) -> String` for strings with placeholders (e.g. `builds_failed: fn(u64, u64) -> String`).
- Two statics, `EN: Texts` and `ZH_TW: Texts`, and `Lang::texts(self) -> &'static Texts`.

A struct literal must set every field, so adding a field without a Chinese value fails `cargo build`; this is how the spec's "Translation missing" scenario is enforced. Call sites read `t.settings` instead of `"Settings"`, with no macro or string key lookup.

*Alternatives:* `rust-i18n` / `fluent` catch missing keys only at runtime or through an extra lint step, add YAML/FTL files and a macro layer, and pay off with dozens of languages and translators outside the codebase. With ≈80 strings and two languages, that is overhead with no benefit.

### 2. Thread `&Texts` through, keep `Display` English

- `kpi::kpis` / `kpi::dora_kpis` / `kpi::delta` take `t: &Texts`. Existing tests pass `&EN` and keep their English assertions; new tests run the same fixtures with `&ZH_TW`.
- `ApiError` keeps its English `Display` for logs, and gains `fn describe(&self, t: &Texts) -> String` for the UI. Server-provided detail (`NotFound(msg)`, `Status(code, msg)`…) is inserted unchanged.
- `DashboardApp` holds the current `Lang`; each frame reads `lang.texts()`. Switching language is only a field change: no refetch and no restart, because every string is derived per frame.

### 3. Font: load a system CJK font at startup, as a fallback family

At startup, `main.rs` looks for a Traditional Chinese capable font and, if found, appends it to the end of both the `Proportional` and `Monospace` families in `egui::FontDefinitions`. Appending keeps the current Latin look and uses the CJK font only for glyphs the default fonts lack, which covers Chinese data under the English UI too.

Lookup order is a fixed list of candidate families per OS, resolved with the `fontdb` crate (pure Rust, reads `.ttc` faces and reports the face index needed by `FontData`):

| OS | Candidates, in order |
|---|---|
| macOS | PingFang TC, Heiti TC, Hiragino Sans CNS |
| Windows | Microsoft JhengHei, Microsoft JhengHei UI |
| Linux | Noto Sans CJK TC, Noto Sans TC, Source Han Sans TC, WenQuanYi Zen Hei |

When none is found the app starts anyway, and the settings panel shows a notice (spec: "No CJK font available").

*Alternative: embed Noto Sans TC in the binary.* Deterministic on every OS and in screenshots, but adds several MB to a ≈14 MB binary, and a subset cannot be used because repo names and descriptions are arbitrary text. Kept as the fallback plan if the spike in Open Questions shows the system lookup is unreliable.

### 4. Locale detection with `sys-locale`

`sys-locale::get_locale()` returns a BCP 47 tag on macOS, Windows and Linux. A pure function `lang_from_locale(&str) -> Lang` maps `zh-TW`, `zh-HK`, `zh-MO` and any tag containing `Hant` to `ZhTw`, everything else (including `zh-CN`, `zh-Hans`, `None`) to `En`. Being pure, it is unit-tested without depending on the CI machine's locale.

### 5. Settings and precedence

`Settings` gains `language: Option<Lang>`; `#[serde(default)]` already on the struct keeps existing `desktop.json` files loading. `None` means "not chosen yet, follow the locale".

Precedence for one run: `DEVPULSE_DESKTOP_LANG` (valid value) > `settings.language` > OS locale > English. `Overrides::from_env` gains the language field; like the other overrides it is never written back. Choosing a language in Settings writes `settings.language` immediately.

### 6. Initial zh-TW glossary

Terms follow Taiwan usage and stay consistent across cards, charts and docs:

| English | zh-TW |
|---|---|
| Repositories | 儲存庫 |
| Settings / Refresh / Apply / This month | 設定 / 重新整理 / 套用 / 本月 |
| CI failure rate | CI 失敗率 |
| Builds per PR | 每個 PR 的建置次數 |
| PR lead time | PR 前置時間 |
| Review wait | 等待審查時間 |
| PR size distribution | PR 大小分布 |
| Daily build duration | 每日建置時間 |
| Deployment frequency | 部署頻率 |
| Lead time for changes | 變更前置時間 |
| Change failure rate | 變更失敗率 |
| Recovery time | 復原時間 |
| MoM | 較上月 |
| ideal / lower is better / higher is better | 理想值 / 越低越好 / 越高越好 |

The full table is reviewed as part of implementation; `desktop/README.zh-TW.md` uses the same terms.

## Risks / Trade-offs

- [macOS PingFang sits in an unstable `AssetsV2` path that `fontdb`'s system scan may not reach] → fall through to Heiti TC / Hiragino Sans CNS in `/System/Library/Fonts/`; confirm which face is actually picked in the spike.
- [System CJK fonts are large (STHeiti Medium.ttc is ≈55 MB) and loading them costs startup time and memory] → load only the one chosen face's file once; measure startup in the spike; switch to the embedded-font plan if it is noticeably slow.
- [Rendering differs per OS, so screenshots and visual checks are not reproducible] → accepted; the tests cover text, not pixels.
- [Chinese strings are longer or wider than English and may overflow KPI cards or the top bar] → check the layout at the minimum window size (800×560) in both languages.
- [Passing `&Texts` through every render function touches most of `app.rs`] → purely mechanical; done in one pass with clippy and the existing tests as the safety net.

## Migration Plan

No data migration. Old `desktop.json` files without `language` load with `None` and follow the locale. Rollback is reverting the change; a newer settings file with a `language` key still loads in the old build because unknown fields are ignored by serde by default.

## Spike result (task 1)

Measured with `fontdb 0.24` and a release build of a throwaway egui window:

| Platform | Found | `find_cjk()` time | Renders |
|---|---|---|---|
| macOS 26 (Apple Silicon, this dev machine) | PingFang TC (`fontdb` also scans `/System/Library/AssetsV2`) | ≈780 ms on the first run after boot, ≈30–45 ms afterwards | Yes, proportional and monospace, Latin text unchanged |
| Windows | Not checked: no Windows machine available | — | — |
| Linux | Not checked: no Linux desktop available | — | — |

Decision 3 stays: system lookup. The untested platforms are covered by the "No CJK font available" path (the app starts, Settings explains), and by the per-OS font notes in the desktop README.

## Open Questions

- Windows and Linux font lookup is unverified (see Spike result); check on real machines before relying on it there.
- Should the zh-TW glossary be reviewed by someone other than the implementer before merge?
