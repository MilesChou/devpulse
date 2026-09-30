//! Dashboard state and the rules for applying API results to it. No
//! egui here: the UI in `app.rs` renders this state and turns clicks
//! into requests, and this module decides what a response changes.

use crate::api::{
    ApiError, ByMember, Member, MemberRow, MonthlyReport, Overview, Registration, Repo, RepoPatch,
    RepoRow, Report, ScopeParam, SyncStatus, Target, Team,
};
use crate::i18n::Texts;
use crate::month::Month;
use crate::notice::{Label, Notice};

/// The shortest span the trend charts cover, ending at the window's end.
pub const TREND_MONTHS: i32 = 12;

/// The longest span the trend charts cover. Matches the server's limit
/// for `metrics/monthly` (`MaxMonths` in `internal/metrics`), which
/// rejects wider requests.
pub const MAX_TREND_MONTHS: i32 = 120;

/// A value fetched in the background.
#[derive(Debug, Clone, PartialEq, Default)]
pub enum Loadable<T> {
    #[default]
    Idle,
    Loading,
    Ready(T),
    Failed(ApiError),
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
            Err(e) => Self::Failed(e),
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
    /// that end where this window ends, or the whole window when it is
    /// longer, so a multi-year selection is charted in full, up to the
    /// last `MAX_TREND_MONTHS` months.
    pub fn trend(self) -> Self {
        Self {
            from: self
                .from
                .min(self.to.add(-TREND_MONTHS))
                .max(self.to.add(-MAX_TREND_MONTHS)),
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

/// A period the top bar offers in one click, relative to the current
/// month. "This year" is the whole calendar year, so its later months
/// are empty until they happen; "last 12 months" includes the current
/// month.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Preset {
    ThisMonth,
    LastMonth,
    ThisYear,
    LastTwelveMonths,
    LastYear,
}

impl Preset {
    pub const ALL: [Preset; 5] = [
        Preset::ThisMonth,
        Preset::LastMonth,
        Preset::ThisYear,
        Preset::LastTwelveMonths,
        Preset::LastYear,
    ];

    pub fn window(self, now: Month) -> Window {
        match self {
            Self::ThisMonth => Window::single(now),
            Self::LastMonth => Window::single(now.prev()),
            Self::ThisYear => Window {
                from: now.january(),
                to: now.january().add(12),
            },
            Self::LastTwelveMonths => Window {
                from: now.add(-11),
                to: now.next(),
            },
            Self::LastYear => Window {
                from: now.january().add(-12),
                to: now.january(),
            },
        }
    }

    /// The preset `w` equals, for labelling the picker; the first match
    /// wins (in January "this year" is also "this month").
    pub fn matching(w: Window, now: Month) -> Option<Preset> {
        Self::ALL.into_iter().find(|p| p.window(now) == w)
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
    /// A member or team was deleted; carries the notice to show on
    /// success.
    PeopleChanged(Notice, Result<(), ApiError>),
    /// The member form was saved; on success the form is cleared, on
    /// failure it is kept so the user can fix it.
    MemberSaved(Notice, Result<(), ApiError>),
    TeamSaved(Notice, Result<(), ApiError>),
    ExcludedSaved(Result<Vec<String>, ApiError>),
    RepoOverview(u64, Result<Overview<RepoRow>, ApiError>),
    MemberOverview(u64, Result<Overview<MemberRow>, ApiError>),
}

/// What the app should do after a message was applied.
#[derive(Debug, Default, PartialEq)]
pub struct Applied {
    pub reload_repos: bool,
    pub reload_metrics: bool,
    pub reload_people: bool,
    pub clear_member_form: bool,
    pub clear_team_form: bool,
    /// A message for the user.
    pub notice: Option<Notice>,
}

impl Applied {
    fn notice(notice: Notice) -> Self {
        Self {
            notice: Some(notice),
            ..Default::default()
        }
    }
}

#[derive(Debug)]
pub struct State {
    pub repos: Loadable<Vec<Repo>>,
    /// What the Dashboard shows: one repo, or all repos.
    pub selected: Option<Target>,
    pub window: Window,
    pub report: Loadable<Report>,
    pub trend: Loadable<MonthlyReport>,
    /// Result of the last connection test, shown in the settings panel.
    pub health: Loadable<()>,
    /// The server's background sync. `Failed` holds why syncing is
    /// unavailable (e.g. no GITHUB_TOKEN on the server), as
    /// `ApiError::Unavailable`.
    pub sync: Loadable<SyncStatus>,
    /// Whose work the dashboard shows.
    pub scope: ScopeParam,
    /// Per-member breakdown of the selected window (everyone scope only).
    pub by_member: Loadable<ByMember>,
    pub members: Loadable<Vec<Member>>,
    pub teams: Loadable<Vec<Team>>,
    pub excluded: Loadable<Vec<String>>,
    /// The Overview's comparison tables, for `window`.
    pub repo_overview: Loadable<Overview<RepoRow>>,
    pub member_overview: Loadable<Overview<MemberRow>>,
    generation: u64,
    overview_generation: u64,
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
            repo_overview: Loadable::Idle,
            member_overview: Loadable::Idle,
            generation: 0,
            overview_generation: 0,
        }
    }

    /// Label for a scope, from the loaded members and teams.
    pub fn scope_label(&self, scope: &ScopeParam, t: &Texts) -> String {
        match scope {
            ScopeParam::Everyone => t.everyone.into(),
            ScopeParam::Member(id) => self
                .members
                .ready()
                .and_then(|ms| ms.iter().find(|m| &m.id == id))
                .map_or_else(|| t.member.into(), |m| m.display_name.clone()),
            ScopeParam::Team(id) => self
                .teams
                .ready()
                .and_then(|ts| ts.iter().find(|team| &team.id == id))
                .map_or_else(|| t.team_fallback.into(), |team| (t.team_label)(&team.name)),
            ScopeParam::Account(a) => a.clone(),
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
        // The breakdown splits everyone's work of one repo; it has no
        // meaning inside a member or team scope, and across all repos the
        // Overview's member table is the breakdown.
        let one_repo = matches!(self.selected, Some(Target::Repo(_)));
        self.by_member = if one_repo && self.scope == ScopeParam::Everyone {
            Loadable::Loading
        } else {
            Loadable::Idle
        };
        self.generation
    }

    /// Marks the Overview tables as loading for the current window and
    /// returns the generation their requests must carry.
    pub fn begin_overview_load(&mut self) -> u64 {
        self.overview_generation += 1;
        self.repo_overview = Loadable::Loading;
        self.member_overview = Loadable::Loading;
        self.overview_generation
    }

    /// Selects a repo or all repos; returns false when it was already
    /// selected.
    pub fn select(&mut self, target: Target) -> bool {
        if self.selected.as_ref() == Some(&target) {
            return false;
        }
        self.selected = Some(target);
        true
    }

    /// The selected repo, when one repo (not all) is selected.
    pub fn selected_repo(&self) -> Option<&Repo> {
        self.selected.as_ref().and_then(Target::repo)
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
                        ..Applied::notice(Notice::RepoRemoved(name))
                    },
                    Err(e) => Applied::notice(Notice::RepoRemoveFailed(name, e)),
                };
            }
            Msg::Sync(r) => return self.sync_update(r),
            Msg::Repos(r) => {
                // Keep the selection only if the refreshed list still has it,
                // matched by name so a re-registered repo is picked up.
                if let (Ok(repos), Some(Target::Repo(sel))) = (&r, &self.selected) {
                    self.selected = repos
                        .iter()
                        .find(|x| x.full_name == sel.full_name)
                        .cloned()
                        .map(Target::Repo);
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
            Msg::RepoOverview(generation, r) if generation == self.overview_generation => {
                self.repo_overview = r.into()
            }
            Msg::MemberOverview(generation, r) if generation == self.overview_generation => {
                self.member_overview = r.into()
            }
            Msg::Report(..)
            | Msg::Trend(..)
            | Msg::ByMember(..)
            | Msg::RepoOverview(..)
            | Msg::MemberOverview(..) => {} // stale
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
            Msg::PeopleChanged(done, r) => return people_changed(done, r),
            Msg::MemberSaved(done, r) => {
                let ok = r.is_ok();
                return Applied {
                    clear_member_form: ok,
                    ..people_changed(done, r)
                };
            }
            Msg::TeamSaved(done, r) => {
                let ok = r.is_ok();
                return Applied {
                    clear_team_form: ok,
                    ..people_changed(done, r)
                };
            }
            Msg::ExcludedSaved(r) => {
                return match r {
                    Ok(accounts) => {
                        self.excluded = Loadable::Ready(accounts);
                        Applied {
                            reload_metrics: true,
                            ..Applied::notice(Notice::ExcludedSaved)
                        }
                    }
                    Err(e) => Applied::notice(Notice::ExcludedSaveFailed(e)),
                };
            }
        }
        Applied::default()
    }

    fn repo_updated(&mut self, r: Result<Repo, ApiError>) -> Applied {
        let repo = match r {
            Ok(repo) => repo,
            Err(e) => return Applied::notice(Notice::RepoSettingsFailed(e)),
        };
        // Labels feed the DORA section, so refresh the metrics when the
        // edited repo is the one on screen.
        let on_screen = self
            .selected_repo()
            .is_some_and(|s| s.full_name == repo.full_name);
        if on_screen {
            self.selected = Some(Target::Repo(repo.clone()));
        }
        Applied {
            reload_repos: true,
            reload_metrics: on_screen,
            ..Applied::notice(Notice::RepoSettingsSaved(repo.full_name.clone()))
        }
    }

    fn sync_update(&mut self, r: Result<SyncStatus, ApiError>) -> Applied {
        let status = match r {
            Ok(status) => status,
            Err(e @ ApiError::Unavailable(_)) => {
                self.sync = Loadable::Failed(e);
                return Applied::default();
            }
            Err(e) => return Applied::notice(Notice::SyncRequestFailed(e)),
        };

        let was_running = self.sync_running();
        self.sync = Loadable::Ready(status.clone());
        if !was_running || !status.running.is_empty() {
            return Applied::default();
        }

        // A sync we were watching just finished. Any repo's new data
        // changes the all-repos numbers.
        let on_screen = match &self.selected {
            Some(Target::All) => true,
            Some(Target::Repo(s)) => s.full_name == status.last_repo,
            None => false,
        };
        let notice = if status.last_error.is_empty() {
            Notice::Synced(status.last_repo.clone())
        } else {
            Notice::SyncFailed {
                repo: status.last_repo.clone(),
                err: status.last_error.clone(),
            }
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

/// A change to members or teams: names and memberships feed the
/// breakdown and the scopes, so both reload on success.
fn people_changed(done: Notice, r: Result<(), ApiError>) -> Applied {
    match r {
        Ok(()) => Applied {
            reload_people: true,
            reload_metrics: true,
            ..Applied::notice(done)
        },
        Err(e) => Applied::notice(Notice::PeopleChangeFailed(e)),
    }
}

fn registered(r: Result<Registration, ApiError>) -> Applied {
    let reg = match r {
        Ok(reg) => reg,
        Err(e) => return Applied::notice(Notice::RepoAddFailed(e)),
    };
    let repo = reg.repo.full_name.clone();
    let notice = match (&reg.metadata_error, reg.created) {
        (Some(err), _) => Notice::RepoAddedWithoutMetadata {
            repo,
            err: err.clone(),
        },
        (None, true) => Notice::RepoAdded(repo),
        (None, false) => Notice::RepoAlreadyTracked(repo),
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
    pub fn validate(&self) -> Result<(String, Vec<String>), Notice> {
        let name = self.name.trim();
        if name.is_empty() {
            return Err(Notice::DisplayNameBlank);
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

    pub fn validate(&self) -> Result<(String, Vec<String>), Notice> {
        let name = self.name.trim();
        if name.is_empty() {
            return Err(Notice::TeamNameBlank);
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
    pub fn to_patch(&self, original: &Repo) -> Result<RepoPatch, Notice> {
        let pr_start: u32 = self
            .pr_start
            .trim()
            .parse()
            .ok()
            .filter(|n| *n >= 1)
            .ok_or_else(|| Notice::PrStartInvalid(self.pr_start.clone()))?;
        let label = |which: Label, v: &str| {
            let v = v.trim();
            if v.is_empty() {
                Err(Notice::LabelBlank(which))
            } else {
                Ok(v.to_string())
            }
        };
        let incident = label(Label::Incident, &self.incident_label)?;
        let hotfix = label(Label::Hotfix, &self.hotfix_label)?;
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
    use crate::i18n::{EN, ZH_TW};

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
        // Shorter than a year: the trend still covers twelve months.
        assert_eq!(
            wide.trend(),
            Window {
                from: m("2025-04"),
                to: m("2026-04")
            }
        );

        // Exactly twelve months: the trend is the window itself.
        let year = Window {
            from: m("2025-04"),
            to: m("2026-04"),
        };
        assert_eq!(year.trend(), year);

        // Longer than a year: the whole window is charted.
        let years = Window {
            from: m("2020-09"),
            to: m("2026-10"),
        };
        assert_eq!(years.trend(), years);

        // Beyond the server's limit: the last MAX_TREND_MONTHS months.
        let decades = Window {
            from: m("2000-01"),
            to: m("2026-10"),
        };
        assert_eq!(
            decades.trend(),
            Window {
                from: m("2016-10"),
                to: m("2026-10")
            }
        );
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
        assert_eq!(s.trend, Loadable::Failed(ApiError::Unauthorized));
    }

    #[test]
    fn repo_refresh_keeps_or_clears_selection() {
        let mut s = State::new(m("2026-05"));
        assert!(s.select(Target::Repo(repo("a/one"))));
        assert!(
            !s.select(Target::Repo(repo("a/one"))),
            "reselecting is a no-op"
        );

        s.apply(Msg::Repos(Ok(vec![repo("a/one"), repo("a/two")])));
        assert_eq!(s.selected.as_ref().map(Target::key), Some("a/one"));

        s.report = Loadable::Loading;
        s.apply(Msg::Repos(Ok(vec![repo("a/two")])));
        assert!(s.selected.is_none());
        assert_eq!(s.report, Loadable::Idle);

        // A failed refresh leaves the selection alone.
        s.select(Target::Repo(repo("a/two")));
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
        let n = a.notice.unwrap();
        assert!(!n.is_error());
        assert_eq!(
            n.text(&EN),
            "Added acme/web. Its data arrives with the next sync."
        );
        assert_eq!(n.text(&ZH_TW), "已新增 acme/web，資料會在下次同步時進來。");

        let a = s.apply(Msg::Registered(Ok(reg("acme/web", false, None))));
        assert_eq!(
            a.notice,
            Some(Notice::RepoAlreadyTracked("acme/web".into()))
        );

        let a = s.apply(Msg::Registered(Ok(reg("acme/x", true, Some("404")))));
        let n = a.notice.unwrap();
        let text = n.text(&EN);
        assert!(
            n.is_error() && text.contains("404") && a.reload_repos,
            "{text}"
        );

        let a = s.apply(Msg::Registered(Err(ApiError::BadRequest(
            "bad name".into(),
        ))));
        assert!(!a.reload_repos);
        assert!(a.notice.unwrap().is_error());
    }

    #[test]
    fn updating_the_selected_repo_reloads_metrics() {
        let mut s = State::new(m("2026-05"));
        s.select(Target::Repo(repo("acme/web")));

        let mut updated = repo("acme/web");
        updated.hotfix_label = "urgent".into();
        let a = s.apply(Msg::RepoUpdated(Ok(updated)));
        assert!(a.reload_repos && a.reload_metrics);
        assert_eq!(s.selected_repo().unwrap().hotfix_label, "urgent");

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
        s.select(Target::Repo(repo("acme/web")));

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
        assert_eq!(a.notice, Some(Notice::Synced("acme/web".into())));
        assert!(!s.sync_running());

        s.apply(Msg::Sync(Ok(status("acme/api", "acme/web", ""))));
        let a = s.apply(Msg::Sync(Ok(status("", "acme/api", "rate limited"))));
        assert!(
            a.reload_repos && !a.reload_metrics,
            "acme/api is not on screen"
        );
        let n = a.notice.unwrap();
        assert!(n.is_error());
        assert_eq!(n.text(&EN), "Sync of acme/api failed: rate limited");

        let a = s.apply(Msg::Sync(Err(ApiError::Conflict(
            "another sync is running: acme/api".into(),
        ))));
        assert!(a.notice.unwrap().is_error());

        s.apply(Msg::Sync(Err(ApiError::Unavailable(
            "no GITHUB_TOKEN".into(),
        ))));
        assert_eq!(
            s.sync,
            Loadable::Failed(ApiError::Unavailable("no GITHUB_TOKEN".into()))
        );
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
        let err = edit.to_patch(&original).unwrap_err();
        assert_eq!(err, Notice::LabelBlank(Label::Incident));
        assert_eq!(err.text(&EN), "Incident label must not be blank");
        assert_eq!(err.text(&ZH_TW), "事故標籤不可空白");
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
        s.select(Target::Repo(repo("a/one")));
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
        assert_eq!(s.scope_label(&s.scope, &EN), "Alice");
        assert_eq!(s.scope_label(&ScopeParam::Everyone, &ZH_TW), "所有人");
    }

    #[test]
    fn people_changes_reload_people_and_metrics() {
        let mut s = State::new(m("2026-05"));
        let saved = Notice::MemberSaved("Alice".into());
        let a = s.apply(Msg::PeopleChanged(saved.clone(), Ok(())));
        assert!(a.reload_people && a.reload_metrics);
        assert_eq!(a.notice, Some(saved.clone()));

        let a = s.apply(Msg::PeopleChanged(
            saved.clone(),
            Err(ApiError::Conflict(
                "account \"bob\" already belongs to Bob".into(),
            )),
        ));
        assert!(!a.reload_people);
        assert!(a.notice.unwrap().is_error());

        let a = s.apply(Msg::MemberSaved(saved.clone(), Ok(())));
        assert!(a.clear_member_form && !a.clear_team_form && a.reload_people);
        let a = s.apply(Msg::MemberSaved(
            saved,
            Err(ApiError::Conflict("display name taken".into())),
        ));
        assert!(!a.clear_member_form, "keep the form so the user can fix it");
        let a = s.apply(Msg::TeamSaved(Notice::TeamSaved("Web".into()), Ok(())));
        assert!(a.clear_team_form);

        let a = s.apply(Msg::ExcludedSaved(Ok(vec!["dependabot".into()])));
        assert!(a.reload_metrics);
        assert_eq!(s.excluded, Loadable::Ready(vec!["dependabot".into()]));
    }

    #[test]
    fn all_repos_target() {
        let mut s = State::new(m("2026-05"));
        s.apply(Msg::Repos(Ok(vec![repo("a/one")])));
        assert!(s.select(Target::All));
        assert!(!s.select(Target::All), "already selected");
        assert_eq!(s.selected_repo(), None);

        // A refreshed repo list keeps All selected.
        s.apply(Msg::Repos(Ok(vec![repo("a/two")])));
        assert_eq!(s.selected, Some(Target::All));

        // No per-repo breakdown across all repos.
        s.begin_metrics_load();
        assert_eq!(s.by_member, Loadable::Idle);
        s.select(Target::Repo(repo("a/two")));
        s.begin_metrics_load();
        assert!(s.by_member.is_loading());
    }

    #[test]
    fn stale_overview_is_dropped() {
        let mut s = State::new(m("2026-05"));
        let first = s.begin_overview_load();
        let second = s.begin_overview_load();
        s.apply(Msg::RepoOverview(first, Err(ApiError::Unauthorized)));
        assert!(
            s.repo_overview.is_loading(),
            "older window's answer is ignored"
        );
        s.apply(Msg::RepoOverview(second, Err(ApiError::Unauthorized)));
        assert_eq!(s.repo_overview, Loadable::Failed(ApiError::Unauthorized));
    }

    #[test]
    fn sync_of_any_repo_reloads_all_repos() {
        let mut s = State::new(m("2026-05"));
        s.select(Target::All);
        let running = SyncStatus {
            running: "acme/web".into(),
            ..Default::default()
        };
        s.apply(Msg::Sync(Ok(running)));
        let done = SyncStatus {
            last_repo: "acme/web".into(),
            ..Default::default()
        };
        assert!(s.apply(Msg::Sync(Ok(done))).reload_metrics);
    }

    #[test]
    fn presets() {
        let now = m("2026-09");
        let w = |from: &str, to: &str| Window {
            from: m(from),
            to: m(to),
        };
        assert_eq!(Preset::ThisMonth.window(now), w("2026-09", "2026-10"));
        assert_eq!(Preset::LastMonth.window(now), w("2026-08", "2026-09"));
        assert_eq!(Preset::ThisYear.window(now), w("2026-01", "2027-01"));
        assert_eq!(
            Preset::LastTwelveMonths.window(now),
            w("2025-10", "2026-10")
        );
        assert_eq!(Preset::LastYear.window(now), w("2025-01", "2026-01"));

        // January: last month and last year cross the year boundary.
        let jan = m("2027-01");
        assert_eq!(Preset::LastMonth.window(jan), w("2026-12", "2027-01"));
        assert_eq!(Preset::LastYear.window(jan), w("2026-01", "2027-01"));
        assert_eq!(Preset::ThisYear.window(jan), w("2027-01", "2028-01"));
        // December: the last 12 months are this year.
        let dec = m("2026-12");
        assert_eq!(
            Preset::matching(Preset::ThisYear.window(dec), dec),
            Some(Preset::ThisYear)
        );
        assert_eq!(
            Preset::LastTwelveMonths.window(dec),
            Preset::ThisYear.window(dec)
        );

        assert_eq!(Preset::matching(w("2026-04", "2026-06"), now), None);
        assert_eq!(
            Preset::matching(w("2025-10", "2026-10"), now),
            Some(Preset::LastTwelveMonths)
        );
    }
}
