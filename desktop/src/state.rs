//! Dashboard state and the rules for applying API results to it. No
//! egui here: the UI in `app.rs` renders this state and turns clicks
//! into requests, and this module decides what a response changes.

use crate::api::{
    ApiError, ByMember, Member, MonthlyReport, Registration, Repo, RepoPatch, Report, ScopeParam,
    SyncStatus, Team,
};
use crate::month::Month;

/// How many months the trend charts cover, ending at the window's end.
pub const TREND_MONTHS: i32 = 12;

/// A value fetched in the background.
#[derive(Debug, Clone, PartialEq, Default)]
pub enum Loadable<T> {
    #[default]
    Idle,
    Loading,
    Ready(T),
    Failed(String),
}

impl<T> Loadable<T> {
    pub fn ready(&self) -> Option<&T> {
        match self {
            Self::Ready(v) => Some(v),
            _ => None,
        }
    }

    pub fn is_loading(&self) -> bool {
        matches!(self, Self::Loading)
    }
}

impl<T> From<Result<T, ApiError>> for Loadable<T> {
    fn from(r: Result<T, ApiError>) -> Self {
        match r {
            Ok(v) => Self::Ready(v),
            Err(e) => Self::Failed(e.to_string()),
        }
    }
}

/// A `[from, to)` month window.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Window {
    pub from: Month,
    pub to: Month,
}

impl Window {
    pub fn single(month: Month) -> Self {
        Self {
            from: month,
            to: month.next(),
        }
    }

    pub fn is_single_month(&self) -> bool {
        self.from.next() == self.to
    }

    /// Moves the whole window by `n` months, keeping its width.
    pub fn shift(self, n: i32) -> Self {
        Self {
            from: self.from.add(n),
            to: self.to.add(n),
        }
    }

    /// The window the trend charts request: the `TREND_MONTHS` months
    /// that end where this window ends.
    pub fn trend(self) -> Self {
        Self {
            from: self.to.add(-TREND_MONTHS),
            to: self.to,
        }
    }

    pub fn label(&self) -> String {
        if self.is_single_month() {
            self.from.to_string()
        } else {
            format!("{} ~ {}", self.from, self.to.prev())
        }
    }
}

/// A finished background request. Metric results carry the generation
/// they were issued under, so a slow response for a previous selection
/// cannot overwrite the current one.
#[derive(Debug)]
pub enum Msg {
    Health(Result<(), ApiError>),
    Repos(Result<Vec<Repo>, ApiError>),
    Report(u64, Result<Report, ApiError>),
    Trend(u64, Result<MonthlyReport, ApiError>),
    Registered(Result<Registration, ApiError>),
    RepoUpdated(Result<Repo, ApiError>),
    /// Carries the removed repo's `owner/name`.
    RepoRemoved(String, Result<(), ApiError>),
    Sync(Result<SyncStatus, ApiError>),
    ByMember(u64, Result<ByMember, ApiError>),
    Members(Result<Vec<Member>, ApiError>),
    Teams(Result<Vec<Team>, ApiError>),
    Excluded(Result<Vec<String>, ApiError>),
    /// A member or team was created, updated or deleted; carries the
    /// message to show on success.
    PeopleChanged(String, Result<(), ApiError>),
    ExcludedSaved(Result<Vec<String>, ApiError>),
}

/// What the app should do after a message was applied.
#[derive(Debug, Default, PartialEq)]
pub struct Applied {
    pub reload_repos: bool,
    pub reload_metrics: bool,
    pub reload_people: bool,
    /// A message for the user: (is_error, text).
    pub notice: Option<(bool, String)>,
}

impl Applied {
    fn notice(is_error: bool, text: impl Into<String>) -> Self {
        Self {
            notice: Some((is_error, text.into())),
            ..Default::default()
        }
    }
}

#[derive(Debug)]
pub struct State {
    pub repos: Loadable<Vec<Repo>>,
    pub selected: Option<Repo>,
    pub window: Window,
    pub report: Loadable<Report>,
    pub trend: Loadable<MonthlyReport>,
    /// Result of the last connection test, shown in the settings panel.
    pub health: Loadable<()>,
    /// The server's background sync. `Failed` holds why syncing is
    /// unavailable (e.g. no GITHUB_TOKEN on the server).
    pub sync: Loadable<SyncStatus>,
    /// Whose work the dashboard shows.
    pub scope: ScopeParam,
    /// Per-member breakdown of the selected window (everyone scope only).
    pub by_member: Loadable<ByMember>,
    pub members: Loadable<Vec<Member>>,
    pub teams: Loadable<Vec<Team>>,
    pub excluded: Loadable<Vec<String>>,
    generation: u64,
}

impl State {
    pub fn new(current: Month) -> Self {
        Self {
            repos: Loadable::Idle,
            selected: None,
            window: Window::single(current),
            report: Loadable::Idle,
            trend: Loadable::Idle,
            health: Loadable::Idle,
            sync: Loadable::Idle,
            scope: ScopeParam::Everyone,
            by_member: Loadable::Idle,
            members: Loadable::Idle,
            teams: Loadable::Idle,
            excluded: Loadable::Idle,
            generation: 0,
        }
    }

    /// Label for a scope, from the loaded members and teams.
    pub fn scope_label(&self, scope: &ScopeParam) -> String {
        match scope {
            ScopeParam::Everyone => "Everyone".into(),
            ScopeParam::Member(id) => self
                .members
                .ready()
                .and_then(|ms| ms.iter().find(|m| &m.id == id))
                .map_or_else(|| "Member".into(), |m| m.display_name.clone()),
            ScopeParam::Team(id) => self
                .teams
                .ready()
                .and_then(|ts| ts.iter().find(|t| &t.id == id))
                .map_or_else(|| "Team".into(), |t| format!("Team {}", t.name)),
        }
    }

    /// Whether the server reported a sync in progress, so the app should
    /// keep polling its status.
    pub fn sync_running(&self) -> bool {
        self.sync.ready().is_some_and(|s| !s.running.is_empty())
    }

    /// Marks the metrics as loading for a new selection or window and
    /// returns the generation the new requests must carry.
    pub fn begin_metrics_load(&mut self) -> u64 {
        self.generation += 1;
        self.report = Loadable::Loading;
        self.trend = Loadable::Loading;
        // The breakdown splits everyone's work; it has no meaning inside
        // a member or team scope.
        self.by_member = if self.scope == ScopeParam::Everyone {
            Loadable::Loading
        } else {
            Loadable::Idle
        };
        self.generation
    }

    /// Selects a repo; returns false when it was already selected.
    pub fn select(&mut self, repo: Repo) -> bool {
        if self.selected.as_ref() == Some(&repo) {
            return false;
        }
        self.selected = Some(repo);
        true
    }

    pub fn apply(&mut self, msg: Msg) -> Applied {
        match msg {
            Msg::Health(r) => self.health = r.into(),
            Msg::Registered(r) => return registered(r),
            Msg::RepoUpdated(r) => return self.repo_updated(r),
            Msg::RepoRemoved(name, r) => {
                return match r {
                    Ok(()) => Applied {
                        reload_repos: true,
                        ..Applied::notice(false, format!("Removed {name} and its synced data."))
                    },
                    Err(e) => Applied::notice(true, format!("Could not remove {name}: {e}")),
                };
            }
            Msg::Sync(r) => return self.sync_update(r),
            Msg::Repos(r) => {
                // Keep the selection only if the refreshed list still has it,
                // matched by name so a re-registered repo is picked up.
                if let (Ok(repos), Some(sel)) = (&r, &self.selected) {
                    self.selected = repos.iter().find(|x| x.full_name == sel.full_name).cloned();
                    if self.selected.is_none() {
                        self.report = Loadable::Idle;
                        self.trend = Loadable::Idle;
                    }
                }
                self.repos = r.into();
            }
            Msg::Report(generation, r) if generation == self.generation => self.report = r.into(),
            Msg::Trend(generation, r) if generation == self.generation => self.trend = r.into(),
            Msg::ByMember(generation, r) if generation == self.generation => {
                self.by_member = r.into()
            }
            Msg::Report(..) | Msg::Trend(..) | Msg::ByMember(..) => {} // stale
            Msg::Members(r) => {
                // Drop a member scope whose member no longer exists.
                if let (Ok(members), ScopeParam::Member(id)) = (&r, &self.scope)
                    && !members.iter().any(|m| &m.id == id)
                {
                    self.scope = ScopeParam::Everyone;
                    self.members = r.into();
                    return Applied {
                        reload_metrics: true,
                        ..Default::default()
                    };
                }
                self.members = r.into();
            }
            Msg::Teams(r) => {
                if let (Ok(teams), ScopeParam::Team(id)) = (&r, &self.scope)
                    && !teams.iter().any(|t| &t.id == id)
                {
                    self.scope = ScopeParam::Everyone;
                    self.teams = r.into();
                    return Applied {
                        reload_metrics: true,
                        ..Default::default()
                    };
                }
                self.teams = r.into();
            }
            Msg::Excluded(r) => self.excluded = r.into(),
            Msg::PeopleChanged(done, r) => {
                return match r {
                    // Names and memberships feed the breakdown and scopes.
                    Ok(()) => Applied {
                        reload_people: true,
                        reload_metrics: true,
                        ..Applied::notice(false, done)
                    },
                    Err(e) => Applied::notice(true, e.to_string()),
                };
            }
            Msg::ExcludedSaved(r) => {
                return match r {
                    Ok(accounts) => {
                        self.excluded = Loadable::Ready(accounts);
                        Applied {
                            reload_metrics: true,
                            ..Applied::notice(false, "Saved excluded accounts.")
                        }
                    }
                    Err(e) => {
                        Applied::notice(true, format!("Could not save excluded accounts: {e}"))
                    }
                };
            }
        }
        Applied::default()
    }

    fn repo_updated(&mut self, r: Result<Repo, ApiError>) -> Applied {
        let repo = match r {
            Ok(repo) => repo,
            Err(e) => return Applied::notice(true, format!("Could not save settings: {e}")),
        };
        // Labels feed the DORA section, so refresh the metrics when the
        // edited repo is the one on screen.
        let on_screen = self
            .selected
            .as_ref()
            .is_some_and(|s| s.full_name == repo.full_name);
        if on_screen {
            self.selected = Some(repo.clone());
        }
        Applied {
            reload_repos: true,
            reload_metrics: on_screen,
            ..Applied::notice(false, format!("Saved settings for {}.", repo.full_name))
        }
    }

    fn sync_update(&mut self, r: Result<SyncStatus, ApiError>) -> Applied {
        let status = match r {
            Ok(status) => status,
            Err(ApiError::Unavailable(msg)) => {
                self.sync = Loadable::Failed(msg);
                return Applied::default();
            }
            Err(ApiError::Conflict(msg)) => return Applied::notice(true, msg),
            Err(e) => return Applied::notice(true, format!("Sync: {e}")),
        };

        let was_running = self.sync_running();
        self.sync = Loadable::Ready(status.clone());
        if !was_running || !status.running.is_empty() {
            return Applied::default();
        }

        // A sync we were watching just finished.
        let on_screen = self
            .selected
            .as_ref()
            .is_some_and(|s| s.full_name == status.last_repo);
        let notice = if status.last_error.is_empty() {
            (false, format!("Synced {}.", status.last_repo))
        } else {
            (
                true,
                format!("Sync of {} failed: {}", status.last_repo, status.last_error),
            )
        };
        Applied {
            reload_repos: true,
            reload_metrics: on_screen,
            notice: Some(notice),
            ..Default::default()
        }
    }

    /// The report of the month before a single-month window, taken from
    /// the loaded trend, for month-over-month deltas.
    pub fn previous_month(&self) -> Option<&Report> {
        if !self.window.is_single_month() {
            return None;
        }
        let months = &self.trend.ready()?.months;
        let prev = self.window.from.prev().to_string();
        months.iter().find(|r| r.from == prev)
    }
}

fn registered(r: Result<Registration, ApiError>) -> Applied {
    let reg = match r {
        Ok(reg) => reg,
        Err(e) => return Applied::notice(true, format!("Could not add the repo: {e}")),
    };
    let name = &reg.repo.full_name;
    let notice = match (&reg.metadata_error, reg.created) {
        (Some(err), _) => (
            true,
            format!(
                "Added {name}, but GitHub metadata could not be fetched ({err}). Check the name, or the server's GITHUB_TOKEN."
            ),
        ),
        (None, true) => (
            false,
            format!("Added {name}. Its data arrives with the next sync."),
        ),
        (None, false) => (false, format!("{name} is already tracked.")),
    };
    Applied {
        reload_repos: true,
        notice: Some(notice),
        ..Default::default()
    }
}

/// Splits free-form account input ("alice, alice-work\nbob") into
/// accounts. The server normalizes and validates them.
pub fn parse_accounts(input: &str) -> Vec<String> {
    input
        .split(|c: char| c == ',' || c.is_whitespace())
        .filter(|s| !s.is_empty())
        .map(str::to_string)
        .collect()
}

/// The member form: creating when `id` is `None`, else editing.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct MemberForm {
    pub id: Option<String>,
    pub name: String,
    pub accounts: String,
}

impl MemberForm {
    pub fn edit(m: &Member) -> Self {
        Self {
            id: Some(m.id.clone()),
            name: m.display_name.clone(),
            accounts: m.accounts.join(", "),
        }
    }

    /// Prefills a new member for an unmapped account.
    pub fn for_account(account: &str) -> Self {
        Self {
            id: None,
            name: String::new(),
            accounts: account.to_string(),
        }
    }

    /// The name and accounts to send, or what is wrong.
    pub fn validate(&self) -> Result<(String, Vec<String>), String> {
        let name = self.name.trim();
        if name.is_empty() {
            return Err("Display name must not be blank".into());
        }
        Ok((name.to_string(), parse_accounts(&self.accounts)))
    }
}

/// The team form: creating when `id` is `None`, else editing.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct TeamForm {
    pub id: Option<String>,
    pub name: String,
    pub member_ids: std::collections::BTreeSet<String>,
}

impl TeamForm {
    pub fn edit(t: &Team) -> Self {
        Self {
            id: Some(t.id.clone()),
            name: t.name.clone(),
            member_ids: t.member_ids.iter().cloned().collect(),
        }
    }

    pub fn validate(&self) -> Result<(String, Vec<String>), String> {
        let name = self.name.trim();
        if name.is_empty() {
            return Err("Team name must not be blank".into());
        }
        Ok((name.to_string(), self.member_ids.iter().cloned().collect()))
    }
}

/// The settings form for one repo, as the user types it.
#[derive(Debug, Clone, PartialEq)]
pub struct RepoEdit {
    pub full_name: String,
    pub pr_start: String,
    pub incident_label: String,
    pub hotfix_label: String,
}

impl RepoEdit {
    pub fn from_repo(r: &Repo) -> Self {
        Self {
            full_name: r.full_name.clone(),
            pr_start: r.pr_start.to_string(),
            incident_label: r.incident_label.clone(),
            hotfix_label: r.hotfix_label.clone(),
        }
    }

    /// Builds the patch of the fields that changed, or says what is wrong
    /// with the input. The server validates too; checking here gives the
    /// message before a round trip.
    pub fn to_patch(&self, original: &Repo) -> Result<RepoPatch, String> {
        let pr_start: u32 = self
            .pr_start
            .trim()
            .parse()
            .ok()
            .filter(|n| *n >= 1)
            .ok_or_else(|| {
                format!(
                    "PR start must be a whole number >= 1, got {:?}",
                    self.pr_start
                )
            })?;
        let label = |name: &str, v: &str| {
            let v = v.trim();
            if v.is_empty() {
                Err(format!("{name} must not be blank"))
            } else {
                Ok(v.to_string())
            }
        };
        let incident = label("Incident label", &self.incident_label)?;
        let hotfix = label("Hotfix label", &self.hotfix_label)?;
        Ok(RepoPatch {
            pr_start: (pr_start != original.pr_start).then_some(pr_start),
            incident_label: (incident != original.incident_label).then_some(incident),
            hotfix_label: (hotfix != original.hotfix_label).then_some(hotfix),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const GOLDEN: &str = include_str!("../../internal/http/testdata/metrics.json");
    const GOLDEN_MONTHLY: &str = include_str!("../../internal/http/testdata/metrics_monthly.json");

    fn m(s: &str) -> Month {
        s.parse().unwrap()
    }

    fn repo(name: &str) -> Repo {
        let (owner, n) = name.split_once('/').unwrap();
        Repo {
            id: format!("id-{name}"),
            full_name: name.into(),
            owner: owner.into(),
            name: n.into(),
            provider: "github".into(),
            description: None,
            default_branch: "main".into(),
            disabled: false,
            pr_start: 1,
            incident_label: "incident".into(),
            hotfix_label: "hotfix".into(),
        }
    }

    #[test]
    fn window_math() {
        let w = Window::single(m("2026-05"));
        assert!(w.is_single_month());
        assert_eq!(w.label(), "2026-05");
        assert_eq!(w.shift(-1), Window::single(m("2026-04")));
        assert_eq!(
            w.trend(),
            Window {
                from: m("2025-06"),
                to: m("2026-06")
            }
        );

        let wide = Window {
            from: m("2026-01"),
            to: m("2026-04"),
        };
        assert!(!wide.is_single_month());
        assert_eq!(wide.label(), "2026-01 ~ 2026-03");
        assert_eq!(wide.shift(2).label(), "2026-03 ~ 2026-05");
    }

    #[test]
    fn drops_stale_metric_responses() {
        let mut s = State::new(m("2026-05"));
        let first = s.begin_metrics_load();
        let second = s.begin_metrics_load();

        let report: Report = serde_json::from_str(GOLDEN).unwrap();
        s.apply(Msg::Report(first, Ok(report.clone())));
        assert!(s.report.is_loading(), "stale response must be ignored");

        s.apply(Msg::Report(second, Ok(report)));
        assert!(s.report.ready().is_some());

        s.apply(Msg::Trend(second, Err(ApiError::Unauthorized)));
        assert_eq!(
            s.trend,
            Loadable::Failed("API token was rejected (401)".into())
        );
    }

    #[test]
    fn repo_refresh_keeps_or_clears_selection() {
        let mut s = State::new(m("2026-05"));
        assert!(s.select(repo("a/one")));
        assert!(!s.select(repo("a/one")), "reselecting is a no-op");

        s.apply(Msg::Repos(Ok(vec![repo("a/one"), repo("a/two")])));
        assert_eq!(
            s.selected.as_ref().map(|r| r.full_name.as_str()),
            Some("a/one")
        );

        s.report = Loadable::Loading;
        s.apply(Msg::Repos(Ok(vec![repo("a/two")])));
        assert!(s.selected.is_none());
        assert_eq!(s.report, Loadable::Idle);

        // A failed refresh leaves the selection alone.
        s.select(repo("a/two"));
        s.apply(Msg::Repos(Err(ApiError::Transport("down".into()))));
        assert!(s.selected.is_some());
        assert!(matches!(s.repos, Loadable::Failed(_)));
    }

    #[test]
    fn previous_month_comes_from_trend() {
        let mut s = State::new(m("2026-05"));
        let generation = s.begin_metrics_load();
        let monthly: MonthlyReport = serde_json::from_str(GOLDEN_MONTHLY).unwrap();
        s.apply(Msg::Trend(generation, Ok(monthly)));

        assert_eq!(s.previous_month().map(|r| r.from.as_str()), Some("2026-04"));

        s.window = Window {
            from: m("2026-04"),
            to: m("2026-06"),
        };
        assert!(
            s.previous_month().is_none(),
            "no delta for multi-month windows"
        );

        s.window = Window::single(m("2026-04"));
        assert!(s.previous_month().is_none(), "March is not in the trend");
    }

    fn reg(name: &str, created: bool, metadata_error: Option<&str>) -> Registration {
        Registration {
            repo: repo(name),
            created,
            metadata_error: metadata_error.map(Into::into),
        }
    }

    #[test]
    fn registration_notices() {
        let mut s = State::new(m("2026-05"));
        let a = s.apply(Msg::Registered(Ok(reg("acme/web", true, None))));
        assert!(a.reload_repos);
        assert_eq!(
            a.notice,
            Some((
                false,
                "Added acme/web. Its data arrives with the next sync.".into()
            ))
        );

        let a = s.apply(Msg::Registered(Ok(reg("acme/web", false, None))));
        assert_eq!(
            a.notice,
            Some((false, "acme/web is already tracked.".into()))
        );

        let a = s.apply(Msg::Registered(Ok(reg("acme/x", true, Some("404")))));
        let (is_error, text) = a.notice.unwrap();
        assert!(is_error && text.contains("404") && a.reload_repos, "{text}");

        let a = s.apply(Msg::Registered(Err(ApiError::BadRequest(
            "bad name".into(),
        ))));
        assert!(!a.reload_repos);
        assert_eq!(a.notice.map(|n| n.0), Some(true));
    }

    #[test]
    fn updating_the_selected_repo_reloads_metrics() {
        let mut s = State::new(m("2026-05"));
        s.select(repo("acme/web"));

        let mut updated = repo("acme/web");
        updated.hotfix_label = "urgent".into();
        let a = s.apply(Msg::RepoUpdated(Ok(updated)));
        assert!(a.reload_repos && a.reload_metrics);
        assert_eq!(s.selected.as_ref().unwrap().hotfix_label, "urgent");

        let a = s.apply(Msg::RepoUpdated(Ok(repo("acme/api"))));
        assert!(a.reload_repos && !a.reload_metrics);
    }

    fn status(running: &str, last: &str, err: &str) -> SyncStatus {
        SyncStatus {
            running: running.into(),
            last_repo: last.into(),
            last_error: err.into(),
            ..Default::default()
        }
    }

    #[test]
    fn sync_lifecycle() {
        let mut s = State::new(m("2026-05"));
        s.select(repo("acme/web"));

        // Idle status on connect: nothing to announce.
        assert_eq!(
            s.apply(Msg::Sync(Ok(status("", "", "")))),
            Applied::default()
        );
        assert!(!s.sync_running());

        assert_eq!(
            s.apply(Msg::Sync(Ok(status("acme/web", "", "")))),
            Applied::default()
        );
        assert!(s.sync_running());
        // Still running: keep quiet.
        assert_eq!(
            s.apply(Msg::Sync(Ok(status("acme/web", "", "")))),
            Applied::default()
        );

        let a = s.apply(Msg::Sync(Ok(status("", "acme/web", ""))));
        assert!(a.reload_repos && a.reload_metrics);
        assert_eq!(a.notice, Some((false, "Synced acme/web.".into())));
        assert!(!s.sync_running());

        s.apply(Msg::Sync(Ok(status("acme/api", "acme/web", ""))));
        let a = s.apply(Msg::Sync(Ok(status("", "acme/api", "rate limited"))));
        assert!(
            a.reload_repos && !a.reload_metrics,
            "acme/api is not on screen"
        );
        assert_eq!(
            a.notice,
            Some((true, "Sync of acme/api failed: rate limited".into()))
        );

        let a = s.apply(Msg::Sync(Err(ApiError::Conflict(
            "another sync is running: acme/api".into(),
        ))));
        assert_eq!(a.notice.map(|n| n.0), Some(true));

        s.apply(Msg::Sync(Err(ApiError::Unavailable(
            "no GITHUB_TOKEN".into(),
        ))));
        assert_eq!(s.sync, Loadable::Failed("no GITHUB_TOKEN".into()));
    }

    #[test]
    fn repo_edit_patch() {
        let original = repo("acme/web");
        let mut edit = RepoEdit::from_repo(&original);
        assert_eq!(
            edit.to_patch(&original),
            Ok(RepoPatch::default()),
            "nothing changed"
        );

        edit.pr_start = " 500 ".into();
        edit.hotfix_label = " urgent ".into();
        assert_eq!(
            edit.to_patch(&original),
            Ok(RepoPatch {
                pr_start: Some(500),
                incident_label: None,
                hotfix_label: Some("urgent".into()),
            })
        );

        for bad in ["0", "-1", "abc", ""] {
            edit.pr_start = bad.into();
            assert!(edit.to_patch(&original).is_err(), "pr_start {bad:?}");
        }
        edit.pr_start = "1".into();
        edit.incident_label = "  ".into();
        assert!(
            edit.to_patch(&original)
                .unwrap_err()
                .contains("Incident label")
        );
    }

    fn member(id: &str, name: &str) -> Member {
        Member {
            id: id.into(),
            display_name: name.into(),
            accounts: vec![name.to_lowercase()],
            team_ids: vec![],
        }
    }

    #[test]
    fn parses_account_input() {
        assert_eq!(
            parse_accounts(" alice, alice-work\nbob  ,, "),
            vec!["alice", "alice-work", "bob"]
        );
        assert!(parse_accounts("  ").is_empty());
    }

    #[test]
    fn member_and_team_forms() {
        let m = member("01M", "Alice");
        let form = MemberForm::edit(&m);
        assert_eq!(form.validate(), Ok(("Alice".into(), vec!["alice".into()])));
        assert!(MemberForm::default().validate().is_err(), "blank name");
        assert_eq!(MemberForm::for_account("bob").accounts, "bob");

        let mut team = TeamForm::default();
        assert!(team.validate().is_err(), "blank team name");
        team.name = " Web ".into();
        team.member_ids.insert("01M".into());
        assert_eq!(team.validate(), Ok(("Web".into(), vec!["01M".into()])));
    }

    #[test]
    fn scope_only_loads_breakdown_for_everyone() {
        let mut s = State::new(m("2026-05"));
        s.begin_metrics_load();
        assert!(s.by_member.is_loading());

        s.scope = ScopeParam::Member("01M".into());
        s.begin_metrics_load();
        assert_eq!(s.by_member, Loadable::Idle);
    }

    #[test]
    fn deleted_member_resets_scope() {
        let mut s = State::new(m("2026-05"));
        s.scope = ScopeParam::Member("gone".into());
        let a = s.apply(Msg::Members(Ok(vec![member("01M", "Alice")])));
        assert_eq!(s.scope, ScopeParam::Everyone);
        assert!(a.reload_metrics);

        s.scope = ScopeParam::Member("01M".into());
        let a = s.apply(Msg::Members(Ok(vec![member("01M", "Alice")])));
        assert_eq!(s.scope, ScopeParam::Member("01M".into()));
        assert!(!a.reload_metrics);
        assert_eq!(s.scope_label(&s.scope), "Alice");
    }

    #[test]
    fn people_changes_reload_people_and_metrics() {
        let mut s = State::new(m("2026-05"));
        let a = s.apply(Msg::PeopleChanged("Saved Alice.".into(), Ok(())));
        assert!(a.reload_people && a.reload_metrics);
        assert_eq!(a.notice, Some((false, "Saved Alice.".into())));

        let a = s.apply(Msg::PeopleChanged(
            "x".into(),
            Err(ApiError::Conflict(
                "account \"bob\" already belongs to Bob".into(),
            )),
        ));
        assert!(!a.reload_people);
        assert_eq!(a.notice.map(|n| n.0), Some(true));

        let a = s.apply(Msg::ExcludedSaved(Ok(vec!["dependabot".into()])));
        assert!(a.reload_metrics);
        assert_eq!(s.excluded, Loadable::Ready(vec!["dependabot".into()]));
    }
}
