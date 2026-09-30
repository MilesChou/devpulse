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
    pub pr_lead_detail: fn(p50: f64, p90: f64, count: u64) -> String,
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
    pub deploy_percentiles: fn(p50: f64, p90: f64, count: u64) -> String,
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
    pr_lead_detail: |p50, p90, n| format!("p50 {p50:.1}h · p90 {p90:.1}h · {n} merged PRs"),
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
    deploy_percentiles: |p50, p90, n| format!("p50 {p50:.1}h · p90 {p90:.1}h · {n} deploys"),
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
                      that month, as average, median (p50) and 90th percentile \
                      (p90). A p90 far above p50 means a few PRs wait much longer \
                      than the rest. Ideal: 24h.",
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
                        window: average, with median (p50) and 90th percentile (p90) \
                        below. Ideal: 24h.",
    help_review_wait: "Average hours from a PR becoming ready for review to its first \
                       review, for PRs that became ready in this window and have been \
                       reviewed. Lower is better.",
    help_deployment_frequency: "Deployments per week in this window, where a \
                                deployment is a PR merged into the default branch. \
                                Deploy days counts the distinct days with at least \
                                one deployment. Higher is better.",
    help_lead_time_for_changes: "Hours from the earliest commit of a deployed PR to \
                                 its merge into the default branch, for deployments \
                                 in this window. Lower is better.",
    help_change_failure_rate: "Remediation deployments ÷ all deployments in this \
                               window. A remediation is a revert, or a PR carrying the \
                               hotfix label. Lower is better.",
    help_recovery_time: "Average hours to recover, from two sources: a reverted PR's \
                         merge to the revert's merge, and an issue with the incident \
                         label from opening to closing. Counted in the window where \
                         recovery ended. Lower is better.",
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
    pr_lead_time: "PR 前置時間",
    pr_lead_detail: |p50, p90, n| format!("p50 {p50:.1}h · p90 {p90:.1}h · {n} 個已合併 PR"),
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
    lead_time_for_changes: "變更前置時間",
    deploy_percentiles: |p50, p90, n| format!("p50 {p50:.1}h · p90 {p90:.1}h · {n} 次部署"),
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
    lead_time_trend: "PR 前置時間（小時）",
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
    help_lead_trend: "每個月：當月合併的 PR 從建立到合併的時數，分別畫出平均、\
                      中位數（p50）與第 90 百分位（p90）。p90 遠高於 p50，\
                      表示少數 PR 等得特別久。理想值 24h。",
    help_deploy_trend: "每個月：平均每週的部署次數，部署指 PR 合併進預設分支。\
                        越高越好。滑到長條上可看當月總次數。",
    help_cfr_trend: "每個月：當月部署中屬於補救的比例；補救指 revert，\
                     或帶有 hotfix 標籤的 PR。沒有部署的月份留白。越低越好。",

    help_ci_failure_rate: "這段期間開始的 PR 建置中，失敗次數 ÷ 總次數。\
                           不含分支建置。理想值 0%。",
    help_builds_per_pr: "這段期間開始、且屬於某個 PR 的 CI 建置，平均每個 PR 跑了幾次。\
                         每次 push 都會重跑 CI，所以用它代替 PR 的重推次數。理想值 1。",
    help_pr_lead_time: "這段期間合併的 PR，從建立到合併的時數：大字為平均，\
                        下方為中位數（p50）與第 90 百分位（p90）。理想值 24h。",
    help_review_wait: "這段期間變成可審查、且已經有人審查的 PR，\
                       從可審查到第一次審查的平均時數。越低越好。",
    help_deployment_frequency: "這段期間平均每週的部署次數，部署指 PR 合併進預設分支。\
                                部署日是至少有一次部署的天數。越高越好。",
    help_lead_time_for_changes: "這段期間的部署，從 PR 最早的 commit 到合併進預設分支\
                                 的時數。越低越好。",
    help_change_failure_rate: "這段期間的部署中，補救部署 ÷ 全部部署。\
                               補救指 revert，或帶有 hotfix 標籤的 PR。越低越好。",
    help_recovery_time: "平均復原時數，來源有兩種：被 revert 的 PR 從合併到 revert \
                         合併，以及帶有 incident 標籤的 issue 從開啟到關閉。\
                         以復原結束的時間歸入期間。越低越好。",
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
    fn serde_names() {
        assert_eq!(serde_json::to_string(&Lang::ZhTw).unwrap(), "\"zh-TW\"");
        assert_eq!(serde_json::from_str::<Lang>("\"en\"").unwrap(), Lang::En);
    }
}
