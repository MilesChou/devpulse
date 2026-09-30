# Desktop Localization

## Purpose

The [desktop dashboard](../desktop-dashboard/spec.md) speaks English and Traditional Chinese (Taiwan): which language is used, how the user picks it, what is and is not translated, and that Chinese text renders at all, since egui's bundled fonts have no CJK glyphs.

## Requirements

### Requirement: The dashboard offers English and Traditional Chinese

The dashboard SHALL offer two UI languages: English and Traditional Chinese (Taiwan). Every user-facing string the dashboard itself produces MUST exist in both languages; a string missing from either language MUST fail the build or the test suite rather than fall back silently at runtime.

#### Scenario: Whole UI in Traditional Chinese

- **WHEN** the UI language is Traditional Chinese
- **THEN** the top bar, settings panel, repo list, KPI cards, DORA cards, chart titles and legends, empty-state hints, notices and error messages are shown in Traditional Chinese using Taiwan terminology

#### Scenario: Whole UI in English

- **WHEN** the UI language is English
- **THEN** the dashboard shows the same text it shows today

#### Scenario: Translation missing

- **WHEN** a developer adds a user-facing string in only one language
- **THEN** `cargo build` or `cargo test` fails

### Requirement: Chinese text renders with a CJK font

The dashboard MUST render Traditional Chinese characters as glyphs, not as placeholder boxes, both in its own UI strings and in data from the API (repo names and descriptions).

#### Scenario: Chinese UI strings

- **WHEN** the UI language is Traditional Chinese
- **THEN** every Chinese character on screen is drawn with a CJK-capable font

#### Scenario: Chinese data under the English UI

- **WHEN** the UI language is English and a repo description from the API contains Chinese characters
- **THEN** those characters are drawn with the CJK-capable font as well

#### Scenario: No CJK font available

- **WHEN** no CJK-capable font can be loaded
- **THEN** the dashboard still starts, and the settings panel says that Chinese text cannot be displayed and why

### Requirement: The language is chosen from the OS locale and can be changed

On first launch the dashboard SHALL pick the UI language from the OS locale. The user MUST be able to change it in Settings; the change applies immediately and is remembered across restarts.

#### Scenario: Traditional Chinese locale

- **WHEN** the dashboard starts with no saved language and the OS locale is Traditional Chinese (`zh-TW`, `zh-HK`, `zh-MO`, or any `zh-Hant` locale)
- **THEN** the UI language is Traditional Chinese

#### Scenario: Any other locale

- **WHEN** the dashboard starts with no saved language and the OS locale is anything else, including Simplified Chinese, or cannot be read
- **THEN** the UI language is English

#### Scenario: Switching in Settings

- **WHEN** the user selects a different language in Settings
- **THEN** the whole UI switches without a restart and without refetching data, and the choice is saved to the settings file

#### Scenario: Saved choice wins over the locale

- **WHEN** the settings file holds a language
- **THEN** the dashboard uses it regardless of the OS locale

### Requirement: The language can be overridden for one run

The dashboard SHALL read `DEVPULSE_DESKTOP_LANG` (`en` or `zh-TW`) and use it for that run only, the same way `DEVPULSE_SERVER_URL` and `DEVPULSE_API_TOKEN` apply to one run.

#### Scenario: Scripted launch

- **WHEN** the dashboard starts with `DEVPULSE_DESKTOP_LANG=zh-TW`
- **THEN** the UI is in Traditional Chinese and the settings file is not modified by the override

#### Scenario: Unknown value

- **WHEN** `DEVPULSE_DESKTOP_LANG` holds a value other than `en` or `zh-TW`
- **THEN** the dashboard ignores it and chooses the language as if it were unset

### Requirement: Data and technical identifiers stay untranslated

The dashboard MUST NOT translate data or identifiers the user needs to match against other tools.

#### Scenario: Untranslated text

- **WHEN** the UI language is Traditional Chinese
- **THEN** repo names, `YYYY-MM` months, dates, numbers, the abbreviations CI, PR, DORA, p50 and p90, CLI commands in hints (e.g. `devpulse repo refresh <owner/name>`), environment variable names, and error detail text returned by the server are shown unchanged

