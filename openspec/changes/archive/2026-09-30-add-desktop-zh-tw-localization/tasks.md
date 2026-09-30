## 1. Font spike

- [x] 1.1 Add `fontdb` and write a throwaway loader that resolves the design's candidate list and appends the face to egui's `Proportional` and `Monospace` families
- [x] 1.2 On macOS, record which candidate is found, the file and face index, startup time with and without it, and confirm Chinese renders (screenshot)
- [x] 1.3 Repeat 1.2 on Windows and on Linux (Ubuntu with `fonts-noto-cjk`), or record which platform could not be checked
- [x] 1.4 Decide: keep system lookup, or switch design Decision 3 to embedding Noto Sans TC; update design.md with the measured result

## 2. Localization layer

- [x] 2.1 Create `src/i18n.rs` with `Lang { En, ZhTw }` (serde `"en"` / `"zh-TW"`), `Texts`, and the `EN` / `ZH_TW` statics
- [x] 2.2 Add `sys-locale` and a pure `lang_from_locale(Option<&str>) -> Lang`; unit-test `zh-TW`, `zh-HK`, `zh-MO`, `zh-Hant-TW`, `zh-CN`, `zh-Hans`, `en-US`, and `None`
- [x] 2.3 Add `language: Option<Lang>` to `Settings`; test that a `desktop.json` without the field loads as `None` and that the field round-trips
- [x] 2.4 Add the language to `Overrides::from_env` from `DEVPULSE_DESKTOP_LANG`, ignoring unknown values; implement and unit-test the precedence env > settings > locale > English

## 3. Move strings into `Texts`

- [x] 3.1 `kpi.rs`: take `&Texts` in `kpis`, `dora_kpis`, `delta` and helpers; keep existing tests on `&EN` passing unchanged
- [x] 3.2 `kpi.rs`: add the same fixture tests against `&ZH_TW` (golden report, empty month, DORA with and without deployments)
- [x] 3.3 `api.rs`: add `ApiError::describe(&self, &Texts)`, keeping `Display` English and the server's detail text unchanged; test both languages
- [x] 3.4 `app.rs`: replace every user-facing literal (top bar, settings panel, repo list, dashboard headings, DORA section, charts, legends, hover text, notices) with `Texts` fields; leave commands, env var names and abbreviations as specified
- [x] 3.5 Check no user-facing English literal is left: `rg '"[A-Z][a-z]' desktop/src` shows only tests, identifiers, and intentional exceptions

## 4. Settings UI and fonts

- [x] 4.1 Add a language selector to the settings panel that switches immediately and saves `settings.language`
- [x] 4.2 Wire the font loader chosen in 1.4 into `main.rs`; when no CJK font is found, start anyway and show the notice in the settings panel (both languages)
- [x] 4.3 Review the full zh-TW string table against the design glossary for Taiwan terminology and consistency

## 5. Verify

- [x] 5.1 `make desktop-lint desktop-test` passes
- [x] 5.2 Run against a live `devpulse serve` in both languages, including `DEVPULSE_DESKTOP_LANG=zh-TW` (confirm `desktop.json` is not modified), switching in Settings, the 401 path, and an empty month
  - note: verified on macOS: `DEVPULSE_DESKTOP_LANG=zh-TW` (full dashboard in Chinese, `desktop.json` unchanged) and the 401 path by the implementer; switching language in Settings and the empty-month view by the user.
- [x] 5.3 Check the layout at the minimum window size (800×560) in zh-TW: no clipped KPI cards or top-bar controls
  - note: the user found cards and charts cut off at the right edge after the (i) icons were added: a title row in `ui.horizontal` does not wrap and widened its column. Fixed with `ui.horizontal_wrapped` (`titled_row`), covered by `long_title_wraps_inside_a_narrow_column`. Re-checked and confirmed by the user.

## 6. Docs

- [x] 6.1 `desktop/README.md` and `desktop/README.zh-TW.md` in sync: language setting, `DEVPULSE_DESKTOP_LANG` in the env var table, `language` in "Where settings live", font requirement per OS and a troubleshooting row for missing CJK fonts
- [x] 6.2 Add a zh-TW dashboard screenshot under `docs/images/` and reference it from `desktop/README.zh-TW.md`
- [x] 6.3 Run `openspec validate add-desktop-zh-tw-localization --strict`

## 7. Trend over long windows

- [x] 7.1 `Window::trend()` covers the whole window when it is longer than twelve months; unit-test shorter, exactly twelve, and longer windows
- [x] 7.2 Update the `desktop-dashboard` delta spec and both desktop READMEs
- [x] 7.3 Raise `MaxMonths` in `internal/metrics` from 36 to 120; update its tests, `internal/http` tests, `docs/commands.md` / `docs/commands.zh-TW.md`, and add the `http-api` delta spec
- [x] 7.4 Cap the dashboard's trend request at 120 months (`MAX_TREND_MONTHS`); unit-test a window wider than 120 months

## 8. Chart help icons

- [x] 8.1 Add an (i) icon, painted with egui shapes, next to each of the six chart titles; its hover text explains the chart
- [x] 8.2 Write the six help texts in English and Traditional Chinese from the Go query definitions (`internal/persistence/persister_metrics.go`, `internal/dora`, `internal/pullrequest/change_stats.go`)
- [x] 8.3 Add the "Each chart explains what it shows" requirement to the `desktop-dashboard` delta spec

## 9. KPI help, clickable icons, drag to select

- [x] 9.1 Add the (i) icon with help text to the four CI KPI cards and the four DORA cards (`Kpi::help`), in both languages
- [x] 9.2 Make the icon clickable: a click pins the explanation in a popup that closes on a click outside; hovering still shows the tooltip
- [x] 9.3 Dragging across a trend chart highlights whole months and, on release, sets the period to them; add a caption under the trend heading
- [x] 9.5 Trend charts span every month of the trend on the x axis (`trend_plot`), not only the months with data
- [x] 9.4 Verify in the running app: hover and click the icons, drag across each trend chart
  - note: the user found that hovering showed nothing (clicking worked). The icon now shows its help as soon as it is hovered (`show_tooltip_ui`) instead of relying on `on_hover_text`'s delay, covered by `hovering_the_info_icon_shows_help_immediately`. Re-checked and confirmed by the user, including dragging.
