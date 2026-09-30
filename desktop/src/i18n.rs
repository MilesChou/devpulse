//! UI languages and every string the dashboard itself shows.
//!
//! Each language is one `Texts` struct literal. A struct literal must set
//! every field, so adding a string without translating it fails to
//! compile rather than falling back at runtime. Strings with values in
//! them are plain `fn`s so each language controls its own word order.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Lang {
    #[serde(rename = "en")]
    En,
    #[serde(rename = "zh-TW")]
    ZhTw,
}

impl Lang {
    pub const ALL: [Lang; 2] = [Lang::En, Lang::ZhTw];

    pub fn texts(self) -> &'static Texts {
        match self {
            Lang::En => &EN,
            Lang::ZhTw => &ZH_TW,
        }
    }

    /// The language's own name, shown the same in every UI language so
    /// users can find theirs.
    pub fn native_name(self) -> &'static str {
        match self {
            Lang::En => "English",
            Lang::ZhTw => "正體中文",
        }
    }

    /// Parses `DEVPULSE_DESKTOP_LANG`: `en` or `zh-TW`, ignoring case
    /// and accepting `_` for `-`.
    pub fn parse(s: &str) -> Option<Lang> {
        match s.trim().replace('_', "-").to_ascii_lowercase().as_str() {
            "en" => Some(Lang::En),
            "zh-tw" => Some(Lang::ZhTw),
            _ => None,
        }
    }

    /// Maps an OS locale to a UI language: Traditional Chinese locales
    /// (`zh-TW`, `zh-HK`, `zh-MO`, `zh-Hant*`) to `ZhTw`, anything else,
    /// including Simplified Chinese and an unreadable locale, to `En`.
    /// Accepts BCP 47 (`zh-Hant-TW`) and POSIX (`zh_TW.UTF-8`) forms.
    pub fn from_locale(locale: Option<&str>) -> Lang {
        let Some(locale) = locale else {
            return Lang::En;
        };
        let tag = locale
            .split(['.', '@'])
            .next()
            .unwrap_or_default()
            .replace('_', "-")
            .to_ascii_lowercase();
        let mut parts = tag.split('-');
        if parts.next() != Some("zh") {
            return Lang::En;
        }
        let rest: Vec<&str> = parts.collect();
        if rest.contains(&"hans") {
            return Lang::En;
        }
        if rest
            .iter()
            .any(|p| matches!(*p, "hant" | "tw" | "hk" | "mo"))
        {
            Lang::ZhTw
        } else {
            Lang::En
        }
    }

    /// The language for this run: a valid environment override, else the
    /// saved choice, else the OS locale.
    pub fn resolve(env: Option<Lang>, saved: Option<Lang>, locale: Option<&str>) -> Lang {
        env.or(saved).unwrap_or_else(|| Lang::from_locale(locale))
    }
}

/// Upper-cases the first letter of a server message ("sync is
/// unavailable" reads as a sentence on its own line).
pub fn capitalize(s: &str) -> String {
    let mut chars = s.chars();
    match chars.next() {
        Some(c) => c.to_uppercase().chain(chars).collect(),
        None => String::new(),
    }
}

pub struct Texts {
    // Top bar.
    pub previous_period: &'static str,
    pub next_period: &'static str,
    pub from: &'static str,
    pub to_exclusive: &'static str,
    pub apply: &'static str,
    pub this_month: &'static str,
    pub settings: &'static str,
    pub close_settings: &'static str,
    pub refresh: &'static str,

    // Settings panel.
    pub connection: &'static str,
    pub server_url: &'static str,
    pub api_token: &'static str,
    pub token_in_use_hint: &'static str,
    pub save_and_connect: &'static str,
    pub test: &'static str,
    pub forget_token: &'static str,
    pub connected: &'static str,
    pub keychain_note: &'static str,
    pub language: &'static str,
    pub no_cjk_font: fn(candidates: &str) -> String,

    // Notices.
    pub keychain_read_failed: fn(err: &str) -> String,
    pub url_required: &'static str,
    pub token_saved: &'static str,
    pub token_save_failed: fn(err: &str) -> String,
    pub no_token: &'static str,
    pub token_removed: &'static str,
    pub token_remove_failed: fn(err: &str) -> String,
    pub settings_save_failed: fn(err: &str) -> String,
    pub from_before_to: &'static str,
    pub bad_month: fn(input: &str) -> String,

    // API errors. The server's own detail text is passed through as is.
    pub err_unauthorized: &'static str,
    pub err_not_found: fn(detail: &str) -> String,
    pub err_bad_request: fn(detail: &str) -> String,
    pub err_status: fn(code: u16, detail: &str) -> String,
    pub err_transport: fn(detail: &str) -> String,
    pub err_decode: fn(detail: &str) -> String,

    // Repo list and headings.
    pub repositories: &'static str,
    pub connect_in_settings: &'static str,
    pub no_repos: &'static str,
    pub select_repo: &'static str,
    pub repo_disabled: &'static str,
    pub trend_heading: fn(from: &str, to: &str) -> String,
    pub trend_drag_hint: &'static str,
    pub dora_heading: fn(branch: &str) -> String,
    pub dora_branch_unknown: fn(repo: &str) -> String,

    // KPI cards.
    pub ci_failure_rate: &'static str,
    pub builds_failed: fn(failed: u64, total: u64) -> String,
    pub ideal_zero_pct: &'static str,
    pub builds_per_pr: &'static str,
    pub repush_proxy: &'static str,
    pub ideal_one: &'static str,
    pub pr_lead_time: &'static str,
    /// Under the median: the mean and p90.
    pub pr_lead_detail: fn(avg: f64, p90: f64, count: u64) -> String,
    pub ideal_24h: &'static str,
    pub review_wait: &'static str,
    pub review_wait_detail: fn(count: u64) -> String,
    pub lower_is_better: &'static str,
    pub higher_is_better: &'static str,
    pub mom: fn(delta: &str) -> String,
    pub mom_hover: &'static str,

    // DORA cards.
    pub deployment_frequency: &'static str,
    /// Unit appended to per-week values, e.g. "2.1/wk".
    pub per_week: &'static str,
    pub deploy_detail: fn(deploys: u64, branch: &str, days: u64) -> String,
    pub lead_time_for_changes: &'static str,
    pub deploy_percentiles: fn(avg: f64, p90: f64, count: u64) -> String,
    pub no_deploys_with_data: &'static str,
    pub change_failure_rate: &'static str,
    pub cfr_detail: fn(reverts: u64, hotfixes: u64, label: &str) -> String,
    pub recovery_time: &'static str,
    pub recovery_detail: fn(reverts: u64, incidents: u64, label: &str) -> String,

    // Charts.
    pub pr_size_distribution: &'static str,
    pub small_share: fn(pct: f64) -> String,
    pub no_prs_in_window: &'static str,
    pub series_prs: &'static str,
    pub daily_build_duration: &'static str,
    pub daily_build_caption: &'static str,
    pub day_bar: fn(day: &str, builds: u64) -> String,
    pub series_seconds: &'static str,
    pub ci_failure_trend: &'static str,
    pub series_failure: &'static str,
    pub lead_time_trend: &'static str,
    pub series_avg: &'static str,
    pub deploys_per_week_trend: &'static str,
    pub deploy_bar: fn(month: &str, deploys: u64) -> String,
    pub series_deploys: &'static str,
    pub cfr_trend: &'static str,
    pub series_cfr: &'static str,

    // Chart help, shown when hovering the (i) next to a chart title.
    pub help_size: &'static str,
    pub help_daily: &'static str,
    pub help_failure_trend: &'static str,
    pub help_lead_trend: &'static str,
    pub help_deploy_trend: &'static str,
    pub help_cfr_trend: &'static str,

    // KPI card help, shown when hovering the (i) next to a card title.
    pub help_ci_failure_rate: &'static str,
    pub help_builds_per_pr: &'static str,
    pub help_pr_lead_time: &'static str,
    pub help_review_wait: &'static str,
    pub help_deployment_frequency: &'static str,
    pub help_lead_time_for_changes: &'static str,
    pub help_change_failure_rate: &'static str,
    pub help_recovery_time: &'static str,
    // Errors the server reports in its own words: busy (409) and
    // unavailable (503). The detail is passed through.
    pub err_conflict: fn(detail: &str) -> String,
    pub err_unavailable: fn(detail: &str) -> String,

    // Page switcher.
    pub view_dashboard: &'static str,
    pub view_repos: &'static str,
    pub view_people: &'static str,

    // Whose work is shown.
    pub show: &'static str,
    pub everyone: &'static str,
    pub member: &'static str,
    pub team_fallback: &'static str,
    pub team_label: fn(name: &str) -> String,
    pub dora_everyone_only: &'static str,

    // By-member breakdown.
    pub by_member: &'static str,
    pub by_member_caption: &'static str,
    pub no_activity: &'static str,
    pub col_name: &'static str,
    pub col_prs_opened: &'static str,
    pub col_merged: &'static str,
    pub col_lead_time: &'static str,
    pub col_ci_failures: &'static str,
    pub show_button: &'static str,
    pub show_member_hover: &'static str,
    pub map_button: &'static str,
    pub map_hover: &'static str,

    // Shared buttons.
    pub save: &'static str,
    pub cancel: &'static str,
    pub edit: &'static str,
    pub delete: &'static str,
    pub delete_ellipsis: &'static str,

    // People page.
    pub people: &'static str,
    pub people_intro: &'static str,
    pub members: &'static str,
    pub members_intro: &'static str,
    pub col_accounts: &'static str,
    pub col_teams: &'static str,
    pub col_members: &'static str,
    pub confirm_delete_member: &'static str,
    pub edit_member: &'static str,
    pub new_member: &'static str,
    pub teams: &'static str,
    pub teams_intro: &'static str,
    pub confirm_delete_team: &'static str,
    pub edit_team: &'static str,
    pub new_team: &'static str,
    pub add_members_first: &'static str,
    pub save_team: &'static str,
    pub excluded_accounts: &'static str,
    pub excluded_intro: &'static str,
    pub save_excluded: &'static str,

    // Repos page.
    pub repos_intro: &'static str,
    pub add_repo: &'static str,
    pub add: &'static str,
    pub add_repo_note: &'static str,
    pub no_repos_short: &'static str,
    pub col_repo: &'static str,
    pub col_default_branch: &'static str,
    pub col_pr_start: &'static str,
    pub col_incident_label: &'static str,
    pub col_hotfix_label: &'static str,
    pub confirm_remove_repo: &'static str,
    pub remove: &'static str,
    pub remove_ellipsis: &'static str,
    pub sync: &'static str,
    pub sync_disabled_hover: &'static str,
    /// The server answers 503 to sync requests only when it has no
    /// GITHUB_TOKEN, so the dashboard says that in its own words.
    pub sync_unavailable: &'static str,
    pub syncing: fn(repo: &str, started: &str) -> String,
    pub last_sync_ok: fn(repo: &str, when: &str) -> String,
    pub last_sync_failed: fn(repo: &str, when: &str, err: &str) -> String,

    // Repo, sync and people notices.
    pub repo_added: fn(repo: &str) -> String,
    pub repo_added_no_metadata: fn(repo: &str, err: &str) -> String,
    pub repo_already_tracked: fn(repo: &str) -> String,
    pub repo_add_failed: fn(err: &str) -> String,
    pub repo_settings_saved: fn(repo: &str) -> String,
    pub repo_settings_failed: fn(err: &str) -> String,
    pub repo_removed: fn(repo: &str) -> String,
    pub repo_remove_failed: fn(repo: &str, err: &str) -> String,
    pub pr_start_invalid: fn(input: &str) -> String,
    pub label_blank: fn(label: &str) -> String,
    pub synced: fn(repo: &str) -> String,
    pub sync_failed: fn(repo: &str, err: &str) -> String,
    pub sync_request_failed: fn(err: &str) -> String,
    pub member_saved: fn(name: &str) -> String,
    pub team_saved: fn(name: &str) -> String,
    pub member_deleted: fn(name: &str) -> String,
    pub team_deleted: fn(name: &str) -> String,
    pub display_name_blank: &'static str,
    pub team_name_blank: &'static str,
    pub excluded_saved: &'static str,
    pub excluded_save_failed: fn(err: &str) -> String,
    // Overview page and All repos.
    pub view_overview: &'static str,
    pub all_repos: &'static str,
    pub repo_comparison: &'static str,
    pub member_comparison: &'static str,
    pub compared_with: fn(previous: &str) -> String,
    pub col_build_time: &'static str,
    pub col_trend: &'static str,
    pub sort_hover: &'static str,
    pub unmapped_hover: &'static str,
    pub open_repo_hover: &'static str,
    pub open_member_hover: &'static str,
    pub no_overview_rows: &'static str,
    pub dora_needs_repo: &'static str,
    pub breakdown_on_overview: &'static str,
    pub go_to_overview: &'static str,
    pub help_prs_opened: &'static str,
    pub help_prs_merged: &'static str,
    pub help_build_time: &'static str,
    pub help_trend: &'static str,

    // KPI status against the ideal, in words for the tooltip.
    pub status_on_target: &'static str,
    pub status_near: &'static str,
    pub status_off: &'static str,

    // Short Overview column titles; the full title is in the hover.
    pub short_prs_opened: &'static str,
    pub short_prs_merged: &'static str,
    pub short_lead_time: &'static str,
    pub short_builds_per_pr: &'static str,
    pub short_ci_failure: &'static str,
    pub short_build_time: &'static str,
    pub short_review_wait: &'static str,
    pub short_deploys: &'static str,
    pub collapse_sidebar: &'static str,
    pub expand_sidebar: &'static str,

    // Period presets ("This month" is `this_month`).
    pub preset_last_month: &'static str,
    pub preset_this_year: &'static str,
    pub preset_last_12: &'static str,
    pub preset_last_year: &'static str,
    pub preset_custom: &'static str,
}

pub static EN: Texts = Texts {
    previous_period: "Previous period",
    next_period: "Next period",
    from: "From",
    to_exclusive: "to (exclusive)",
    apply: "Apply",
    this_month: "This month",
    settings: "Settings",
    close_settings: "Close settings",
    refresh: "Refresh",

    connection: "Connection",
    server_url: "DevPulse server URL (`devpulse serve`)",
    api_token: "API token",
    token_in_use_hint: "(in use; leave empty to keep)",
    save_and_connect: "Save & connect",
    test: "Test",
    forget_token: "Forget token",
    connected: "Connected: server is up and the token works.",
    keychain_note: "The token is kept in the OS keychain, one entry per server URL. \
                    GitHub and CI tokens stay on the server.",
    language: "Language",
    no_cjk_font: |c| {
        format!("No Chinese font found (looked for {c}); Chinese text shows as boxes.")
    },

    keychain_read_failed: |e| format!("Cannot read the keychain: {e}"),
    url_required: "Server URL is required.",
    token_saved: "Saved. The token is stored in the OS keychain.",
    token_save_failed: |e| {
        format!("Could not save the token to the keychain ({e}); it is kept for this session only.")
    },
    no_token: "No API token for this server.",
    token_removed: "Token removed from the keychain.",
    token_remove_failed: |e| format!("Could not remove the token: {e}"),
    settings_save_failed: |e| format!("Could not save settings: {e}"),
    from_before_to: "From must be before To.",
    bad_month: |s| format!("expected YYYY-MM, got {s:?}"),

    err_unauthorized: "API token was rejected (401)",
    err_not_found: |m| format!("not found: {m}"),
    err_bad_request: |m| format!("bad request: {m}"),
    err_status: |c, m| format!("server error {c}: {m}"),
    err_transport: |m| format!("cannot reach server: {m}"),
    err_decode: |m| format!("unexpected response: {m}"),

    repositories: "Repositories",
    connect_in_settings: "Connect to a server in Settings.",
    no_repos: "No repos yet. Register one with `devpulse repo add <owner/name>`.",
    select_repo: "Select a repository.",
    repo_disabled: "This repo is disabled upstream; `devpulse sync` skips it.",
    trend_heading: |from, to| format!("Trend · {from} ~ {to}"),
    trend_drag_hint: "Drag across a chart to set the period to those months.",
    dora_heading: |b| format!("DORA · deployment = PR merged into {b}"),
    dora_branch_unknown: |r| {
        format!("Default branch unknown; run `devpulse repo refresh {r}` on the server.")
    },

    ci_failure_rate: "CI failure rate",
    builds_failed: |f, t| format!("{f} / {t} PR builds failed"),
    ideal_zero_pct: "ideal 0%",
    builds_per_pr: "Builds per PR",
    repush_proxy: "re-push proxy: CI runs per PR",
    ideal_one: "ideal 1",
    pr_lead_time: "PR lead time",
    pr_lead_detail: |avg, p90, n| format!("avg {avg:.1}h · p90 {p90:.1}h · {n} merged PRs"),
    ideal_24h: "ideal 24h",
    review_wait: "Review wait",
    review_wait_detail: |n| format!("ready to first review · {n} PRs"),
    lower_is_better: "lower is better",
    higher_is_better: "higher is better",
    mom: |d| format!("{d} MoM"),
    mom_hover: "change vs the previous month",

    deployment_frequency: "Deployment frequency",
    per_week: "/wk",
    deploy_detail: |n, b, days| format!("{n} deploys into {b} · {days} deploy days"),
    lead_time_for_changes: "Lead time for changes",
    deploy_percentiles: |avg, p90, n| format!("avg {avg:.1}h · p90 {p90:.1}h · {n} deploys"),
    no_deploys_with_data: "no deploys with data",
    change_failure_rate: "Change failure rate",
    cfr_detail: |r, h, l| format!("{r} reverts + {h} hotfixes (label \"{l}\")"),
    recovery_time: "Recovery time",
    recovery_detail: |r, i, l| format!("{r} from reverts · {i} incidents (label \"{l}\")"),

    pr_size_distribution: "PR size distribution",
    small_share: |p| format!("{p:.0}% small (XS + S) · ideal: mostly small"),
    no_prs_in_window: "no PRs in this window",
    series_prs: "PRs",
    daily_build_duration: "Daily build duration",
    daily_build_caption: "average seconds per UTC day",
    day_bar: |d, n| format!("{d} ({n} builds)"),
    series_seconds: "seconds",
    ci_failure_trend: "CI failure rate (%)",
    series_failure: "failure %",
    lead_time_trend: "PR lead time (hours)",
    series_avg: "avg",
    deploys_per_week_trend: "Deployments per week",
    deploy_bar: |m, n| format!("{m} ({n} deploys)"),
    series_deploys: "deploys / week",
    cfr_trend: "Change failure rate (%)",
    series_cfr: "change failure %",

    help_size: "PRs opened in this window, grouped by total changed lines \
                (additions + deletions): XS < 50, S < 200, M < 500, L < 1000, \
                XL ≥ 1000. The goal is for most PRs to be XS or S.",
    help_daily: "Average duration of the CI builds that started on each UTC day \
                 of this window, PR and branch builds alike. Hover a bar for the \
                 day's build count. Taller bars mean slower feedback.",
    help_failure_trend: "Per month: the share of PR-triggered builds started that \
                         month that failed. Months without PR builds are left \
                         blank. Ideal: 0%.",
    help_lead_trend: "Per month: hours from PR creation to merge, for PRs merged \
                      that month: the median (p50, the main line), the average \
                      and the 90th percentile (p90). An average or p90 far above \
                      the median means a few PRs wait much longer than the rest. \
                      Ideal: 24h.",
    help_deploy_trend: "Per month: deployments per week, where a deployment is a PR \
                        merged into the default branch. Higher is better. Hover a \
                        bar for the month's total.",
    help_cfr_trend: "Per month: the share of that month's deployments that were \
                     remediations, i.e. a revert or a PR carrying the hotfix \
                     label. Months without deployments are left blank. Lower is \
                     better.",

    help_ci_failure_rate: "Failed builds ÷ all PR-triggered builds started in this \
                           window. Branch builds are not counted. Ideal: 0%.",
    help_builds_per_pr: "Average number of CI builds per PR, over builds started in \
                         this window that belong to a PR. Every push re-runs CI, so \
                         this stands in for how often a PR is re-pushed. Ideal: 1.",
    help_pr_lead_time: "Hours from PR creation to merge, for PRs merged in this \
                        window. The big number is the median, so a few PRs left open \
                        for weeks do not dominate it; the average and the 90th \
                        percentile (p90) are below. Ideal: 24h.",
    help_review_wait: "Average hours from a PR becoming ready for review to its first \
                       review, for PRs that became ready in this window and have been \
                       reviewed. Lower is better.",
    help_deployment_frequency: "Deployments per week in this window, where a \
                                deployment is a PR merged into the default branch. \
                                Deploy days counts the distinct days with at least \
                                one deployment. Higher is better.",
    help_lead_time_for_changes: "DORA's Lead Time for Changes: hours from the earliest \
                                 commit of a deployed PR to its merge into the default \
                                 branch, for deployments in this window. The big number \
                                 is the median; the average and p90 are below. Lower is \
                                 better.",
    help_change_failure_rate: "Remediation deployments ÷ all deployments in this \
                               window. A remediation is a revert, or a PR carrying the \
                               hotfix label. Lower is better.",
    help_recovery_time: "Median hours to recover, from two sources: a reverted PR's \
                         merge to the revert's merge, and an issue with the incident \
                         label from opening to closing. Counted in the window where \
                         recovery ended. Lower is better.",
    err_conflict: |m| capitalize(m),
    err_unavailable: |m| capitalize(m),

    view_dashboard: "Dashboard",
    view_repos: "Repos",
    view_people: "People",

    show: "Show",
    everyone: "Everyone",
    member: "Member",
    team_fallback: "Team",
    team_label: |n| format!("Team {n}"),
    dora_everyone_only: "DORA measures delivery of the whole repo; choose Everyone to see it.",

    by_member: "By member",
    by_member_caption: "Members with activity in this window, and active accounts not mapped \
                        to a member yet. Excluded accounts (bots) are left out.",
    no_activity: "No activity in this window.",
    col_name: "Name",
    col_prs_opened: "PRs opened",
    col_merged: "Merged",
    col_lead_time: "Lead time",
    col_ci_failures: "CI failures",
    show_button: "Show",
    show_member_hover: "Show only this member",
    map_button: "Map…",
    map_hover: "Create a member for this account",

    save: "Save",
    cancel: "Cancel",
    edit: "Edit",
    delete: "Delete",
    delete_ellipsis: "Delete…",

    people: "People",
    people_intro: "Map GitHub accounts to people and teams, and choose which accounts to \
                   leave out of the metrics.",
    members: "Members",
    members_intro: "A member is one person. List every GitHub account they use; accounts \
                    compare case-insensitively.",
    col_accounts: "Accounts",
    col_teams: "Teams",
    col_members: "Members",
    confirm_delete_member: "Delete this member?",
    edit_member: "Edit member",
    new_member: "New member",
    teams: "Teams",
    teams_intro: "A team is a set of members; the dashboard can show a team's work on its own.",
    confirm_delete_team: "Delete this team? Its members stay.",
    edit_team: "Edit team",
    new_team: "New team",
    add_members_first: "Add members first to put them in a team.",
    save_team: "Save team",
    excluded_accounts: "Excluded accounts",
    excluded_intro: "Bots and other accounts left out of every metric except DORA: their PRs, \
                     builds and reviews. One account per line; a trailing [bot] is ignored, \
                     so `dependabot` also covers `dependabot[bot]`.",
    save_excluded: "Save excluded accounts",

    repos_intro: "Add, configure, sync or remove the repos this server tracks.",
    add_repo: "Add repo",
    add: "Add",
    add_repo_note: "GitHub metadata is fetched right away; pull requests and builds arrive \
                    with the next sync.",
    no_repos_short: "No repos yet.",
    col_repo: "Repo",
    col_default_branch: "Default branch",
    col_pr_start: "PR start",
    col_incident_label: "Incident label",
    col_hotfix_label: "Hotfix label",
    confirm_remove_repo: "Delete it and all its synced data?",
    remove: "Remove",
    remove_ellipsis: "Remove…",
    sync: "Sync",
    sync_disabled_hover: "A sync is running, or the server cannot sync",
    sync_unavailable: "This server has no GITHUB_TOKEN, so it cannot sync from the dashboard. \
                       Run `devpulse sync` on the server instead.",
    syncing: |r, s| format!("Syncing {r} (started {s})…"),
    last_sync_ok: |r, w| format!("Last sync: {r} finished {w}."),
    last_sync_failed: |r, w, e| format!("Last sync: {r} failed {w}: {e}"),

    repo_added: |r| format!("Added {r}. Its data arrives with the next sync."),
    repo_added_no_metadata: |r, e| {
        format!(
            "Added {r}, but GitHub metadata could not be fetched ({e}). Check the name, \
             or the server's GITHUB_TOKEN."
        )
    },
    repo_already_tracked: |r| format!("{r} is already tracked."),
    repo_add_failed: |e| format!("Could not add the repo: {e}"),
    repo_settings_saved: |r| format!("Saved settings for {r}."),
    repo_settings_failed: |e| format!("Could not save settings: {e}"),
    repo_removed: |r| format!("Removed {r} and its synced data."),
    repo_remove_failed: |r, e| format!("Could not remove {r}: {e}"),
    pr_start_invalid: |s| format!("PR start must be a whole number >= 1, got {s:?}"),
    label_blank: |l| format!("{l} must not be blank"),
    synced: |r| format!("Synced {r}."),
    sync_failed: |r, e| format!("Sync of {r} failed: {e}"),
    sync_request_failed: |e| format!("Sync: {e}"),
    member_saved: |n| format!("Saved {n}."),
    team_saved: |n| format!("Saved team {n}."),
    member_deleted: |n| format!("Deleted {n}."),
    team_deleted: |n| format!("Deleted team {n}."),
    display_name_blank: "Display name must not be blank",
    team_name_blank: "Team name must not be blank",
    excluded_saved: "Saved excluded accounts.",
    excluded_save_failed: |e| format!("Could not save excluded accounts: {e}"),
    view_overview: "Overview",
    all_repos: "All repos",
    repo_comparison: "Repos",
    member_comparison: "Members",
    compared_with: |p| {
        format!("Changes compare with the previous period of the same length ({p}).")
    },
    col_build_time: "Build time",
    col_trend: "Last 12 months",
    sort_hover: "Click to sort, worst first; click again to reverse",
    unmapped_hover: "Not mapped to a member yet",
    open_repo_hover: "Open this repo's dashboard",
    open_member_hover: "Open this member's dashboard across all repos",
    no_overview_rows: "Nothing to compare in this period.",
    dora_needs_repo: "DORA is measured per repo; pick a repo in the list to see it.",
    breakdown_on_overview: "The per-member breakdown across all repos is on the Overview.",
    go_to_overview: "Open Overview",
    help_prs_opened: "PRs opened in this period.",
    help_prs_merged: "PRs merged in this period.",
    help_build_time: "Average duration of the CI builds that started in this period, per build: \
                      a day with many builds weighs more. PR and branch builds alike.",
    help_trend: "The sorted column for the 12 months ending with this period; gaps are \
                 months without data.",
    status_on_target: "On target",
    status_near: "Near the target",
    status_off: "Off target",

    short_prs_opened: "Opened",
    short_prs_merged: "Merged",
    short_lead_time: "Lead time",
    short_builds_per_pr: "Builds/PR",
    short_ci_failure: "CI fail",
    short_build_time: "Build time",
    short_review_wait: "Review wait",
    short_deploys: "Deploys/wk",
    collapse_sidebar: "Hide the repo list",
    expand_sidebar: "Show the repo list",
    preset_last_month: "Last month",
    preset_this_year: "This year",
    preset_last_12: "Last 12 months",
    preset_last_year: "Last year",
    preset_custom: "Custom",
};

pub static ZH_TW: Texts = Texts {
    previous_period: "上一段期間",
    next_period: "下一段期間",
    from: "從",
    to_exclusive: "到（不含）",
    apply: "套用",
    this_month: "本月",
    settings: "設定",
    close_settings: "關閉設定",
    refresh: "重新整理",

    connection: "連線",
    server_url: "DevPulse 伺服器網址（`devpulse serve`）",
    api_token: "API token",
    token_in_use_hint: "（使用中；留空則沿用）",
    save_and_connect: "儲存並連線",
    test: "測試",
    forget_token: "移除 token",
    connected: "已連線：伺服器正常，token 有效。",
    keychain_note: "token 存在作業系統的鑰匙圈，每個伺服器網址各存一筆。\
                    GitHub 與 CI 的 token 只留在伺服器上。",
    language: "語言",
    no_cjk_font: |c| format!("找不到中文字型（已尋找 {c}），中文會顯示成方塊。"),

    keychain_read_failed: |e| format!("無法讀取鑰匙圈：{e}"),
    url_required: "請輸入伺服器網址。",
    token_saved: "已儲存，token 存放在作業系統的鑰匙圈。",
    token_save_failed: |e| format!("無法將 token 存入鑰匙圈（{e}），這次執行期間仍會使用。"),
    no_token: "這個伺服器沒有 API token。",
    token_removed: "已從鑰匙圈移除 token。",
    token_remove_failed: |e| format!("無法移除 token：{e}"),
    settings_save_failed: |e| format!("無法儲存設定：{e}"),
    from_before_to: "起始月份必須早於結束月份。",
    bad_month: |s| format!("月份格式應為 YYYY-MM，收到 {s:?}"),

    err_unauthorized: "API token 被拒絕（401）",
    err_not_found: |m| format!("找不到：{m}"),
    err_bad_request: |m| format!("請求有誤：{m}"),
    err_status: |c, m| format!("伺服器錯誤 {c}：{m}"),
    err_transport: |m| format!("無法連上伺服器：{m}"),
    err_decode: |m| format!("無法解析回應：{m}"),

    repositories: "儲存庫",
    connect_in_settings: "請先在「設定」連線到伺服器。",
    no_repos: "還沒有儲存庫。用 `devpulse repo add <owner/name>` 新增。",
    select_repo: "請選擇一個儲存庫。",
    repo_disabled: "這個儲存庫在上游已停用，`devpulse sync` 會略過它。",
    trend_heading: |from, to| format!("趨勢 · {from} ~ {to}"),
    trend_drag_hint: "在圖上拖曳，可把期間改成選取的月份。",
    dora_heading: |b| format!("DORA · 部署 = PR 合併進 {b}"),
    dora_branch_unknown: |r| {
        format!("還不知道預設分支，請在伺服器上執行 `devpulse repo refresh {r}`。")
    },

    ci_failure_rate: "CI 失敗率",
    builds_failed: |f, t| format!("{t} 次 PR 建置中失敗 {f} 次"),
    ideal_zero_pct: "理想值 0%",
    builds_per_pr: "每個 PR 的建置次數",
    repush_proxy: "重推次數的替代指標：每個 PR 的 CI 執行次數",
    ideal_one: "理想值 1",
    pr_lead_time: "PR 開啟到合併",
    pr_lead_detail: |avg, p90, n| format!("平均 {avg:.1}h · p90 {p90:.1}h · {n} 個已合併 PR"),
    ideal_24h: "理想值 24h",
    review_wait: "等待審查時間",
    review_wait_detail: |n| format!("從可審查到第一次審查 · {n} 個 PR"),
    lower_is_better: "越低越好",
    higher_is_better: "越高越好",
    mom: |d| format!("較上月 {d}"),
    mom_hover: "與上個月相比的變化",

    deployment_frequency: "部署頻率",
    per_week: "/週",
    deploy_detail: |n, b, days| format!("{n} 次部署到 {b} · {days} 個部署日"),
    lead_time_for_changes: "commit 到部署",
    deploy_percentiles: |avg, p90, n| format!("平均 {avg:.1}h · p90 {p90:.1}h · {n} 次部署"),
    no_deploys_with_data: "沒有可計算的部署",
    change_failure_rate: "變更失敗率",
    cfr_detail: |r, h, l| format!("{r} 次 revert + {h} 次 hotfix（標籤「{l}」）"),
    recovery_time: "復原時間",
    recovery_detail: |r, i, l| format!("{r} 次來自 revert · {i} 次事故（標籤「{l}」）"),

    pr_size_distribution: "PR 大小分布",
    small_share: |p| format!("{p:.0}% 為小型 PR（XS + S）· 理想：以小型為主"),
    no_prs_in_window: "這段期間沒有 PR",
    series_prs: "PR 數",
    daily_build_duration: "每日建置時間",
    daily_build_caption: "每個 UTC 日的平均秒數",
    day_bar: |d, n| format!("{d}（{n} 次建置）"),
    series_seconds: "秒",
    ci_failure_trend: "CI 失敗率（%）",
    series_failure: "失敗 %",
    lead_time_trend: "PR 開啟到合併（小時）",
    series_avg: "平均",
    deploys_per_week_trend: "每週部署次數",
    deploy_bar: |m, n| format!("{m}（{n} 次部署）"),
    series_deploys: "部署次數 / 週",
    cfr_trend: "變更失敗率（%）",
    series_cfr: "變更失敗 %",

    help_size: "這段期間建立的 PR，依總變更行數（新增 + 刪除）分級：\
                XS < 50、S < 200、M < 500、L < 1000、XL ≥ 1000。\
                目標是大部分 PR 落在 XS 或 S。",
    help_daily: "這段期間每個 UTC 日開始的 CI 建置平均耗時，PR 與分支建置都算在內。\
                 滑到長條上可看當天的建置次數。長條越高，代表回饋越慢。",
    help_failure_trend: "每個月：當月開始的 PR 建置中，失敗所佔的比例。\
                         沒有 PR 建置的月份留白。理想值 0%。",
    help_lead_trend: "每個月：當月合併的 PR 從開啟到合併的時數，畫出中位數（p50，主線）、\
                      平均與第 90 百分位（p90）。平均或 p90 遠高於中位數，\
                      表示少數 PR 等得特別久。理想值 24h。",
    help_deploy_trend: "每個月：平均每週的部署次數，部署指 PR 合併進預設分支。\
                        越高越好。滑到長條上可看當月總次數。",
    help_cfr_trend: "每個月：當月部署中屬於補救的比例；補救指 revert，\
                     或帶有 hotfix 標籤的 PR。沒有部署的月份留白。越低越好。",

    help_ci_failure_rate: "這段期間開始的 PR 建置中，失敗次數 ÷ 總次數。\
                           不含分支建置。理想值 0%。",
    help_builds_per_pr: "這段期間開始、且屬於某個 PR 的 CI 建置，平均每個 PR 跑了幾次。\
                         每次 push 都會重跑 CI，所以用它代替 PR 的重推次數。理想值 1。",
    help_pr_lead_time: "這段期間合併的 PR，從開啟到合併的時數。大字是中位數，\
                        不會被少數開了好幾週的 PR 拉高；下方是平均與第 90 百分位（p90）。\
                        理想值 24h。",
    help_review_wait: "這段期間變成可審查、且已經有人審查的 PR，\
                       從可審查到第一次審查的平均時數。越低越好。",
    help_deployment_frequency: "這段期間平均每週的部署次數，部署指 PR 合併進預設分支。\
                                部署日是至少有一次部署的天數。越高越好。",
    help_lead_time_for_changes: "DORA 的 Lead Time for Changes：這段期間的部署，\
                                 從 PR 最早的 commit 到合併進預設分支的時數。\
                                 大字是中位數，下方是平均與 p90。越低越好。",
    help_change_failure_rate: "這段期間的部署中，補救部署 ÷ 全部部署。\
                               補救指 revert，或帶有 hotfix 標籤的 PR。越低越好。",
    help_recovery_time: "復原時數的中位數，來源有兩種：被 revert 的 PR 從合併到 revert \
                         合併，以及帶有 incident 標籤的 issue 從開啟到關閉。\
                         以復原結束的時間歸入期間。越低越好。",
    err_conflict: |m| format!("伺服器正忙：{m}"),
    err_unavailable: |m| format!("伺服器無法執行：{m}"),

    view_dashboard: "儀表板",
    view_repos: "儲存庫管理",
    view_people: "人員",

    show: "顯示",
    everyone: "所有人",
    member: "成員",
    team_fallback: "團隊",
    team_label: |n| format!("團隊 {n}"),
    dora_everyone_only: "DORA 衡量的是整個儲存庫的交付，請在「顯示」選「所有人」才看得到。",

    by_member: "依成員",
    by_member_caption: "這段期間有活動的成員，以及還沒對應到成員的活躍帳號。\
                        已排除的帳號（機器人）不列入。",
    no_activity: "這段期間沒有活動。",
    col_name: "名稱",
    col_prs_opened: "開啟的 PR",
    col_merged: "已合併",
    col_lead_time: "開啟到合併",
    col_ci_failures: "CI 失敗",
    show_button: "顯示",
    show_member_hover: "只顯示這位成員",
    map_button: "對應…",
    map_hover: "為這個帳號建立成員",

    save: "儲存",
    cancel: "取消",
    edit: "編輯",
    delete: "刪除",
    delete_ellipsis: "刪除…",

    people: "人員",
    people_intro: "把 GitHub 帳號對應到人員與團隊，並選擇哪些帳號不列入指標。",
    members: "成員",
    members_intro: "一位成員代表一個人。列出他使用的所有 GitHub 帳號；帳號比對不分大小寫。",
    col_accounts: "帳號",
    col_teams: "團隊",
    col_members: "成員",
    confirm_delete_member: "要刪除這位成員嗎？",
    edit_member: "編輯成員",
    new_member: "新增成員",
    teams: "團隊",
    teams_intro: "團隊是一組成員；儀表板可以只顯示某個團隊的工作。",
    confirm_delete_team: "要刪除這個團隊嗎？成員會保留。",
    edit_team: "編輯團隊",
    new_team: "新增團隊",
    add_members_first: "請先新增成員，才能把他們加入團隊。",
    save_team: "儲存團隊",
    excluded_accounts: "排除的帳號",
    excluded_intro: "機器人和其他不列入指標（DORA 除外）的帳號，它們的 PR、建置與審查都不計算。\
                     一行一個帳號；結尾的 [bot] 會被忽略，所以 `dependabot` 也涵蓋 \
                     `dependabot[bot]`。",
    save_excluded: "儲存排除的帳號",

    repos_intro: "新增、設定、同步或移除這台伺服器追蹤的儲存庫。",
    add_repo: "新增儲存庫",
    add: "新增",
    add_repo_note: "GitHub 的基本資料會立即抓取；PR 和建置要等下次同步才會進來。",
    no_repos_short: "還沒有儲存庫。",
    col_repo: "儲存庫",
    col_default_branch: "預設分支",
    col_pr_start: "PR 起始編號",
    col_incident_label: "事故標籤",
    col_hotfix_label: "Hotfix 標籤",
    confirm_remove_repo: "要刪除它和所有已同步的資料嗎？",
    remove: "移除",
    remove_ellipsis: "移除…",
    sync: "同步",
    sync_disabled_hover: "已有同步在進行，或伺服器無法同步",
    sync_unavailable: "這台伺服器沒有設定 GITHUB_TOKEN，無法從儀表板同步。\
                       請改在伺服器上執行 `devpulse sync`。",
    syncing: |r, s| format!("正在同步 {r}（{s} 開始）…"),
    last_sync_ok: |r, w| format!("上次同步：{r}，{w} 完成。"),
    last_sync_failed: |r, w, e| format!("上次同步：{r}，{w} 失敗：{e}"),

    repo_added: |r| format!("已新增 {r}，資料會在下次同步時進來。"),
    repo_added_no_metadata: |r, e| {
        format!(
            "已新增 {r}，但無法抓取 GitHub 基本資料（{e}）。\
             請檢查名稱，或伺服器的 GITHUB_TOKEN。"
        )
    },
    repo_already_tracked: |r| format!("{r} 已經在追蹤中。"),
    repo_add_failed: |e| format!("無法新增儲存庫：{e}"),
    repo_settings_saved: |r| format!("已儲存 {r} 的設定。"),
    repo_settings_failed: |e| format!("無法儲存設定：{e}"),
    repo_removed: |r| format!("已移除 {r} 及其同步資料。"),
    repo_remove_failed: |r, e| format!("無法移除 {r}：{e}"),
    pr_start_invalid: |s| format!("PR 起始編號必須是大於等於 1 的整數，收到 {s:?}"),
    label_blank: |l| format!("{l}不可空白"),
    synced: |r| format!("已同步 {r}。"),
    sync_failed: |r, e| format!("{r} 同步失敗：{e}"),
    sync_request_failed: |e| format!("同步：{e}"),
    member_saved: |n| format!("已儲存 {n}。"),
    team_saved: |n| format!("已儲存團隊 {n}。"),
    member_deleted: |n| format!("已刪除 {n}。"),
    team_deleted: |n| format!("已刪除團隊 {n}。"),
    display_name_blank: "名稱不可空白",
    team_name_blank: "團隊名稱不可空白",
    excluded_saved: "已儲存排除的帳號。",
    excluded_save_failed: |e| format!("無法儲存排除的帳號：{e}"),
    view_overview: "總覽",
    all_repos: "全部儲存庫",
    repo_comparison: "儲存庫比較",
    member_comparison: "成員比較",
    compared_with: |p| format!("變化是與長度相同的前一段期間（{p}）比較。"),
    col_build_time: "建置時間",
    col_trend: "近 12 個月",
    sort_hover: "點擊排序，最差的在前；再點一次反向",
    unmapped_hover: "還沒有對應到成員",
    open_repo_hover: "開啟這個儲存庫的儀表板",
    open_member_hover: "開啟這位成員在全部儲存庫的儀表板",
    no_overview_rows: "這段期間沒有可以比較的資料。",
    dora_needs_repo: "DORA 是以單一儲存庫計算的，請在清單中選擇一個儲存庫。",
    breakdown_on_overview: "跨儲存庫的成員比較在總覽頁。",
    go_to_overview: "前往總覽",
    help_prs_opened: "這段期間開啟的 PR 數。",
    help_prs_merged: "這段期間合併的 PR 數。",
    help_build_time: "這段期間開始的 CI 建置平均耗時，以每次建置計算：建置多的日子權重較大。\
                      PR 與分支建置都算在內。",
    help_trend: "排序中的欄位在這段期間結尾往前 12 個月的走勢；空白表示那個月沒有資料。",
    status_on_target: "達標",
    status_near: "接近目標",
    status_off: "偏離目標",

    short_prs_opened: "開啟 PR",
    short_prs_merged: "合併",
    short_lead_time: "開到合併",
    short_builds_per_pr: "建置/PR",
    short_ci_failure: "CI 失敗",
    short_build_time: "建置時間",
    short_review_wait: "審查等待",
    short_deploys: "部署/週",
    collapse_sidebar: "收合儲存庫清單",
    expand_sidebar: "展開儲存庫清單",
    preset_last_month: "上月",
    preset_this_year: "今年",
    preset_last_12: "近一年",
    preset_last_year: "去年",
    preset_custom: "自訂",
};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn locale_mapping() {
        for tw in [
            "zh-TW",
            "zh-HK",
            "zh-MO",
            "zh-Hant",
            "zh-Hant-TW",
            "zh-Hant-US",
            "zh_TW.UTF-8",
        ] {
            assert_eq!(Lang::from_locale(Some(tw)), Lang::ZhTw, "{tw}");
        }
        for en in [
            "zh-CN",
            "zh-Hans",
            "zh-Hans-TW",
            "zh",
            "en-US",
            "ja-JP",
            "C",
            "",
        ] {
            assert_eq!(Lang::from_locale(Some(en)), Lang::En, "{en}");
        }
        assert_eq!(Lang::from_locale(None), Lang::En);
    }

    #[test]
    fn parse_env_value() {
        assert_eq!(Lang::parse("en"), Some(Lang::En));
        assert_eq!(Lang::parse("zh-TW"), Some(Lang::ZhTw));
        assert_eq!(Lang::parse(" zh_tw "), Some(Lang::ZhTw));
        assert_eq!(Lang::parse("zh-CN"), None);
        assert_eq!(Lang::parse(""), None);
    }

    #[test]
    fn precedence_env_then_saved_then_locale() {
        use Lang::*;
        assert_eq!(Lang::resolve(Some(En), Some(ZhTw), Some("zh-TW")), En);
        assert_eq!(Lang::resolve(None, Some(En), Some("zh-TW")), En);
        assert_eq!(Lang::resolve(None, None, Some("zh-TW")), ZhTw);
        assert_eq!(Lang::resolve(None, None, None), En);
    }

    #[test]
    fn capitalizes_first_letter() {
        assert_eq!(capitalize("sync is unavailable"), "Sync is unavailable");
        assert_eq!(capitalize(""), "");
        assert_eq!(capitalize("Already"), "Already");
    }

    #[test]
    fn serde_names() {
        assert_eq!(serde_json::to_string(&Lang::ZhTw).unwrap(), "\"zh-TW\"");
        assert_eq!(serde_json::from_str::<Lang>("\"en\"").unwrap(), Lang::En);
    }
}
