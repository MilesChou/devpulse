## 1. Theme

- [x] 1.1 `theme.rs`: accent, corner radii, spacing, panel and card fills for light and dark; replace `apply_text_sizes` with `theme::apply`
- [x] 1.2 `theme::card` frame; put KPI groups, charts, DORA, trends, Overview tables and People / Repos sections on cards
- [x] 1.3 Page switcher as tabs; period controls grouped
- [x] 1.4 Chart palette from the theme (bars, lines, legend)

## 2. KPI status

- [x] 2.1 `kpi.rs`: `Status { OnTarget, Near, Off }`, the design's thresholds, `status` on each card and for the small-PR share; unit-test each threshold edge and "no data, no status"
- [x] 2.2 Card rendering: status bar on the left edge, coloured value, status in the tooltip (both languages)

## 3. Overview

- [x] 3.1 `overview::change` reports `better` beyond the floor; unit-test
- [x] 3.2 Sparkline as the second column; short column titles (full title, help and sort hint on hover) instead of wrapping headers; repo name with its owner on a second line; long names truncated
- [x] 3.3 Change tags with arrow and tinted background
- [x] 3.4 Check both tables fit at 1200 × 820 (window-only screenshot)

## 4. Repo list

- [x] 4.1 Group by owner with headings, names without owner, full name on hover
- [x] 4.2 Owner groups fold; a top-bar button hides the whole list, remembered in `desktop.json` (`sidebar_collapsed`), asked for by the user during review

## 5. Verify

- [x] 5.1 `make desktop-lint desktop-test`
- [x] 5.2 Window-only screenshots of Overview and Dashboard in light and dark mode
  - note: taken with the window id from CoreGraphics and `screencapture -l`, so no other window is captured; the Dashboard and dark shots used a temporary local build (default view, forced theme) that was reverted
- [x] 5.3 `openspec validate polish-dashboard-visuals --strict`

## 6. Period presets

- [x] 6.1 `state::Preset` (this month, last month, this year, last 12 months, last year) with the windows from the spec; unit-test, including January
  - note: "this year" first ran to the current month; the user expects the whole calendar year (2026-01 to 2026-12), which also makes its trend chart cover exactly that year
- [x] 6.2 Replace the "This month" button with a picker that names the matching preset or "Custom"; strings in both languages

## 7. Median lead time and plain names

- [x] 7.1 Overview summaries carry `lead_time_p50_hours` (median) instead of the mean; golden files regenerated; docs updated
- [x] 7.2 Duration cards lead with the median; detail shows mean and p90; status and month-over-month change use the median; unit test with a long tail
- [x] 7.3 Lead-time trend: the median is the main line
- [x] 7.4 zh-TW names "PR 開啟到合併" and "commit 到部署" (help keeps DORA's name); "本年" → "今年"

