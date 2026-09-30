## Context

egui 0.36 styles through `Visuals` (panel and window fills, widget corner radii, selection and hyperlink colours) and `Frame` (fill, stroke, corner radius, margins, shadow). The app sets only text sizes today (`apply_text_sizes`) and uses egui's default grey theme.

## Goals / Non-Goals

**Goals:** a consistent accent and card layout in light and dark mode; status colours that follow the project's stated ideals; Overview tables that fit an ordinary window.

**Non-Goals:** custom fonts (the CJK fallback stays as is), animations, a theme picker.

## Decisions

### 1. One `theme.rs`

`theme::apply(ctx)` sets, for both the light and the dark style: an indigo accent (`#4F6BED` light, `#8EA2FF` dark) for selection, hyperlinks and focus; widget corner radius 6; card corner radius 10; item spacing 8 × 6 and button padding 10 × 5; panel fill a shade off the card fill so cards stand out. `theme::card(ui, …)` wraps a section in a `Frame` with that fill, a hairline stroke and a soft shadow. Status and change colours (`good`, `near`, `bad`) come from the same module so every page agrees.

### 2. KPI status thresholds

The project goals (CLAUDE.md) set an ideal but no tolerance. These initial thresholds are a proposal to confirm; they live in one table in `kpi.rs`:

| Card | Ideal | On target | Near | Off |
|---|---|---|---|---|
| CI failure rate | 0 % | ≤ 5 % | ≤ 15 % | > 15 % |
| Builds per PR | 1 | ≤ 1.5 | ≤ 2.5 | > 2.5 |
| PR lead time | 24 h | ≤ 24 h | ≤ 72 h | > 72 h |
| Small-PR share (XS + S) | mostly small | ≥ 70 % | ≥ 50 % | < 50 % |

A card without data has no status. Review wait and the DORA cards have no ideal and stay neutral. The card shows the status as a coloured bar on its left edge and colours its value; the tooltip names the status in words so colour is not the only signal.

### 3. Overview layout

- Sparkline moves to the second column, right after the name, so it is always visible.
- Headers use short titles ("建置/PR", "CI 失敗"); the full title, its explanation and the sort hint are on hover. Wrapping headers were tried first and made the header row tall and uneven.
- Repo names show the name with the owner on a second, weaker line, in a fixed-width column that truncates long names; the full name is on hover.
- The change becomes a tag: `⏶201%` / `⏷59%` with a tinted background — warning when worse beyond the floor, green when better beyond it, neutral in between. `overview::change` gains `better` alongside `worse`.

### 4. Repo list

Repos are grouped under their owner (sorted), each entry showing the repo name; "All repos" stays on top.

## Risks / Trade-offs

- [Thresholds are opinions] → one table, documented, easy to change; the ideal is still printed on every card.
- [Colour-only signals are hard for colour-blind users] → status also appears in the tooltip, and change tags carry an arrow.
- [Dark mode can drift] → every colour is chosen per `dark_mode` in `theme.rs`, and a screenshot in both modes is part of the tasks.
