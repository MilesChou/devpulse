//! The egui dashboard: renders `State` and runs API calls on worker
//! threads, which report back over a channel.

use std::path::PathBuf;
use std::sync::mpsc::{Receiver, Sender, channel};
use std::thread;
use std::time::{Duration, Instant};

use eframe::egui::{self, Align2, FontId, RichText, Sense, Stroke};
use egui_plot::{
    Bar, BarChart, Legend, Line, Plot, PlotPoints, PlotResponse, log_grid_spacer,
    uniform_grid_spacer,
};

use crate::api::{
    ByMember, Client, MemberRow, MonthlyReport, Repo, RepoRow, Report, ScopeParam, Summary, Target,
};
use crate::i18n::{Lang, Texts};
use crate::kpi::{self, Direction, Kpi};
use crate::month::Month;
use crate::notice::Notice;
use crate::overview::{Column, Sort, change};
use crate::settings::{self, SecretStore, Settings};
use crate::state::{Loadable, MemberForm, Msg, Preset, RepoEdit, State, TeamForm, Window};
use crate::theme::{self, Tone};

/// How often to poll the server while a sync runs.
const SYNC_POLL: Duration = Duration::from_secs(2);

/// The page shown in the central panel.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum View {
    Overview,
    Dashboard,
    Repos,
    People,
}

/// Builds the token store for a server URL.
pub type SecretStoreFactory = Box<dyn Fn(&str) -> Box<dyn SecretStore>>;

/// Session-only values, from `DEVPULSE_SERVER_URL`,
/// `DEVPULSE_API_TOKEN` and `DEVPULSE_DESKTOP_LANG`. They win over the
/// settings file and keychain but are never written to either, so
/// scripted or CI launches leave no trace in the user's keychain.
#[derive(Debug, Default, Clone)]
pub struct Overrides {
    pub base_url: Option<String>,
    pub token: Option<String>,
    /// Ignored when the variable holds an unknown language.
    pub lang: Option<Lang>,
}

impl Overrides {
    pub fn from_env() -> Self {
        let var = |k| {
            std::env::var(k)
                .ok()
                .map(|v| v.trim().to_string())
                .filter(|v| !v.is_empty())
        };
        Self {
            base_url: var("DEVPULSE_SERVER_URL"),
            token: var("DEVPULSE_API_TOKEN"),
            lang: var("DEVPULSE_DESKTOP_LANG").and_then(|v| Lang::parse(&v)),
        }
    }
}

pub struct DashboardApp {
    state: State,
    settings: Settings,
    settings_path: Option<PathBuf>,
    make_store: SecretStoreFactory,
    /// Server URL from the environment; replaces `settings.base_url`
    /// until the user saves a URL in the settings panel.
    url_override: Option<String>,
    /// The token in use this session. Kept even if the keychain write
    /// failed, so the dashboard still works until it is closed.
    token: Option<String>,
    lang: Lang,
    /// The font families searched when no CJK font was found, shown in
    /// the settings panel; `None` when one was loaded.
    missing_cjk_font: Option<String>,

    // Settings form.
    show_settings: bool,
    url_input: String,
    token_input: String,
    notice: Option<Notice>,

    // Window form.
    from_input: String,
    to_input: String,

    // Repos page.
    view: View,
    /// How the Overview's two tables are sorted.
    repo_sort: Sort,
    member_sort: Sort,
    add_input: String,
    editing: Option<RepoEdit>,
    /// `owner/name` awaiting a second click to confirm removal.
    confirm_remove: Option<String>,

    // People page.
    member_form: MemberForm,
    team_form: TeamForm,
    excluded_input: String,
    /// The excluded list as last loaded, to tell user edits apart.
    excluded_shown: String,
    /// Member or team id awaiting a second click to confirm deletion.
    confirm_delete: Option<String>,
    last_sync_poll: Option<Instant>,

    tx: Sender<Msg>,
    rx: Receiver<Msg>,
    ctx: egui::Context,
}

impl DashboardApp {
    pub fn new(
        ctx: egui::Context,
        settings_path: Option<PathBuf>,
        make_store: SecretStoreFactory,
        overrides: Overrides,
        locale: Option<String>,
        missing_cjk_font: Option<String>,
    ) -> Self {
        let settings = settings_path
            .as_deref()
            .map(settings::load)
            .unwrap_or_default();
        let lang = Lang::resolve(overrides.lang, settings.language, locale.as_deref());
        theme::apply(&ctx);
        let state = State::new(Month::current());
        let (tx, rx) = channel();

        let mut app = Self {
            from_input: state.window.from.to_string(),
            to_input: state.window.to.to_string(),
            state,
            url_input: overrides
                .base_url
                .clone()
                .unwrap_or_else(|| settings.base_url.clone()),
            settings,
            settings_path,
            make_store,
            url_override: overrides.base_url,
            token: None,
            lang,
            missing_cjk_font,
            show_settings: false,
            token_input: String::new(),
            notice: None,
            view: View::Overview,
            repo_sort: Sort::new(Column::CiFailureRate),
            member_sort: Sort::new(Column::PrsOpened),
            add_input: String::new(),
            editing: None,
            confirm_remove: None,
            last_sync_poll: None,
            member_form: MemberForm::default(),
            team_form: TeamForm::default(),
            excluded_input: String::new(),
            excluded_shown: String::new(),
            confirm_delete: None,
            tx,
            rx,
            ctx,
        };

        if let Some(token) = overrides.token {
            app.token = Some(token);
            app.load_repos();
            return app;
        }
        match (app.make_store)(app.base_url()).get() {
            Ok(Some(token)) => {
                app.token = Some(token);
                app.load_repos();
            }
            Ok(None) => app.show_settings = true,
            Err(e) => {
                app.show_settings = true;
                app.notice = Some(Notice::KeychainReadFailed(e));
            }
        }
        app
    }

    fn base_url(&self) -> &str {
        self.url_override
            .as_deref()
            .unwrap_or(&self.settings.base_url)
    }

    fn client(&self) -> Option<Client> {
        self.token
            .as_deref()
            .map(|t| Client::new(self.base_url(), t))
    }

    /// Runs `job` on a worker thread and wakes the UI when it is done.
    fn spawn(&self, job: impl FnOnce() -> Msg + Send + 'static) {
        let tx = self.tx.clone();
        let ctx = self.ctx.clone();
        thread::spawn(move || {
            // The receiver only goes away when the app is closing.
            let _ = tx.send(job());
            ctx.request_repaint();
        });
    }

    fn load_repos(&mut self) {
        let Some(client) = self.client() else { return };
        self.state.repos = Loadable::Loading;
        let c = client.clone();
        self.spawn(move || Msg::Repos(c.list_repos()));
        // Also learn whether a sync is running (or possible at all).
        self.spawn(move || Msg::Sync(client.sync_status()));
        self.load_people();
    }

    fn register_repo(&mut self) {
        let Some(client) = self.client() else { return };
        let name = self.add_input.trim().to_string();
        if name.is_empty() {
            return;
        }
        self.add_input.clear();
        self.spawn(move || Msg::Registered(client.register_repo(&name)));
    }

    fn save_repo_edit(&mut self, original: &Repo) {
        let Some(edit) = &self.editing else { return };
        let patch = match edit.to_patch(original) {
            Ok(p) => p,
            Err(e) => {
                self.notice = Some(e);
                return;
            }
        };
        self.editing = None;
        if patch == Default::default() {
            return;
        }
        let Some(client) = self.client() else { return };
        let repo = original.clone();
        self.spawn(move || Msg::RepoUpdated(client.update_repo(&repo, &patch)));
    }

    fn remove_repo(&mut self, repo: &Repo) {
        let Some(client) = self.client() else { return };
        self.confirm_remove = None;
        let repo = repo.clone();
        self.spawn(move || Msg::RepoRemoved(repo.full_name.clone(), client.remove_repo(&repo)));
    }

    fn start_sync(&mut self, repo: &Repo) {
        let Some(client) = self.client() else { return };
        let repo = repo.clone();
        self.last_sync_poll = Some(Instant::now());
        self.spawn(move || Msg::Sync(client.start_sync(&repo)));
    }

    /// While a sync runs, asks the server for its status every
    /// SYNC_POLL, and schedules a repaint so this runs without input.
    fn poll_sync(&mut self) {
        if !self.state.sync_running() {
            return;
        }
        let due = self.last_sync_poll.is_none_or(|t| t.elapsed() >= SYNC_POLL);
        if due && let Some(client) = self.client() {
            self.last_sync_poll = Some(Instant::now());
            self.spawn(move || Msg::Sync(client.sync_status()));
        }
        self.ctx.request_repaint_after(SYNC_POLL);
    }

    fn load_metrics(&mut self) {
        let (Some(client), Some(target)) = (self.client(), self.state.selected.clone()) else {
            return;
        };
        let generation = self.state.begin_metrics_load();
        let window = self.state.window;

        let scope = self.state.scope.clone();

        let (c, tg, s) = (client.clone(), target.clone(), scope.clone());
        self.spawn(move || Msg::Report(generation, c.metrics(&tg, window.from, window.to, &s)));

        if let (Target::Repo(repo), ScopeParam::Everyone) = (&target, &scope) {
            let (c, r) = (client.clone(), repo.clone());
            self.spawn(move || {
                Msg::ByMember(generation, c.metrics_by_member(&r, window.from, window.to))
            });
        }

        let trend = window.trend();
        self.spawn(move || {
            Msg::Trend(
                generation,
                client.monthly_metrics(&target, trend.from, trend.to, &scope),
            )
        });
    }

    /// Loads the Overview's comparison tables for the current window.
    fn load_overview(&mut self) {
        let Some(client) = self.client() else { return };
        let generation = self.state.begin_overview_load();
        let window = self.state.window;
        let c = client.clone();
        self.spawn(move || Msg::RepoOverview(generation, c.repo_overview(window.from, window.to)));
        self.spawn(move || {
            Msg::MemberOverview(generation, client.member_overview(window.from, window.to))
        });
    }

    /// Opens the Dashboard for a target and scope, remembering the target.
    fn open_dashboard(&mut self, target: Target, scope: ScopeParam) {
        self.settings.last_repo = Some(target.key().to_string());
        self.persist_settings();
        self.state.select(target);
        self.state.scope = scope;
        self.view = View::Dashboard;
        self.load_metrics();
    }

    /// Loads members, teams and excluded accounts.
    fn load_people(&mut self) {
        let Some(client) = self.client() else { return };
        let (a, b) = (client.clone(), client.clone());
        self.spawn(move || Msg::Members(a.list_members()));
        self.spawn(move || Msg::Teams(b.list_teams()));
        self.spawn(move || Msg::Excluded(client.excluded_accounts()));
    }

    fn save_member(&mut self) {
        let (name, accounts) = match self.member_form.validate() {
            Ok(v) => v,
            Err(e) => {
                self.notice = Some(e);
                return;
            }
        };
        let Some(client) = self.client() else { return };
        let id = self.member_form.id.clone();
        self.spawn(move || {
            let done = Notice::MemberSaved(name.clone());
            Msg::MemberSaved(
                done,
                client
                    .save_member(id.as_deref(), &name, &accounts)
                    .map(|_| ()),
            )
        });
    }

    fn save_team(&mut self) {
        let (name, member_ids) = match self.team_form.validate() {
            Ok(v) => v,
            Err(e) => {
                self.notice = Some(e);
                return;
            }
        };
        let Some(client) = self.client() else { return };
        let id = self.team_form.id.clone();
        self.spawn(move || {
            let done = Notice::TeamSaved(name.clone());
            Msg::TeamSaved(
                done,
                client
                    .save_team(id.as_deref(), &name, &member_ids)
                    .map(|_| ()),
            )
        });
    }

    fn delete_member(&mut self, id: String, name: String) {
        let Some(client) = self.client() else { return };
        self.confirm_delete = None;
        self.spawn(move || {
            Msg::PeopleChanged(Notice::MemberDeleted(name), client.delete_member(&id))
        });
    }

    fn delete_team(&mut self, id: String, name: String) {
        let Some(client) = self.client() else { return };
        self.confirm_delete = None;
        self.spawn(move || Msg::PeopleChanged(Notice::TeamDeleted(name), client.delete_team(&id)));
    }

    fn save_excluded(&mut self) {
        let Some(client) = self.client() else { return };
        let accounts = crate::state::parse_accounts(&self.excluded_input);
        self.spawn(move || Msg::ExcludedSaved(client.replace_excluded_accounts(&accounts)));
    }

    /// Opens the People page with a new member prefilled for `account`.
    fn map_account(&mut self, account: &str) {
        self.member_form = MemberForm::for_account(account);
        self.view = View::People;
    }

    fn set_window(&mut self, window: Window) {
        self.state.window = window;
        self.from_input = window.from.to_string();
        self.to_input = window.to.to_string();
        self.load_metrics();
        self.load_overview();
    }

    /// Saves the URL to the settings file and the token to the keychain,
    /// then reconnects.
    fn save_settings(&mut self) {
        let url = self.url_input.trim().trim_end_matches('/').to_string();
        if url.is_empty() {
            self.notice = Some(Notice::UrlRequired);
            return;
        }
        let url_changed = url != self.base_url().trim_end_matches('/');
        self.settings.base_url = url;
        self.url_override = None;
        self.persist_settings();

        let typed = self.token_input.trim().to_string();
        let mut store = (self.make_store)(&self.settings.base_url);
        if !typed.is_empty() {
            self.notice = Some(match store.set(&typed) {
                Ok(()) => Notice::TokenSaved,
                Err(e) => Notice::TokenSaveFailed(e),
            });
            self.token_input.clear();
        }
        self.token = settings::token_after_save(self.token.take(), url_changed, &typed, || {
            store.get().ok().flatten()
        });

        if self.token.is_none() {
            self.notice = Some(Notice::NoToken);
            return;
        }
        self.state.selected = None;
        self.state.report = Loadable::Idle;
        self.state.trend = Loadable::Idle;
        self.load_repos();
    }

    fn forget_token(&mut self) {
        let mut store = (self.make_store)(self.base_url());
        self.notice = Some(match store.delete() {
            Ok(()) => Notice::TokenRemoved,
            Err(e) => Notice::TokenRemoveFailed(e),
        });
        self.token = None;
        self.state.repos = Loadable::Idle;
        self.state.selected = None;
    }

    fn test_connection(&mut self) {
        let url = self.url_input.trim().to_string();
        let token = if self.token_input.trim().is_empty() {
            self.token.clone().unwrap_or_default()
        } else {
            self.token_input.trim().to_string()
        };
        self.state.health = Loadable::Loading;
        self.spawn(move || {
            let client = Client::new(&url, &token);
            // /healthz proves the server is up; listing repos proves the
            // token is accepted.
            Msg::Health(
                client
                    .health()
                    .and_then(|()| client.list_repos().map(|_| ())),
            )
        });
    }

    fn persist_settings(&mut self) {
        if let Some(path) = &self.settings_path
            && let Err(e) = settings::save(path, &self.settings)
        {
            self.notice = Some(Notice::SettingsSaveFailed(e.to_string()));
        }
    }

    fn restore_last_repo(&mut self) {
        if self.state.selected.is_some() {
            return;
        }
        let Some(last) = self.settings.last_repo.clone() else {
            return;
        };
        if last == Target::ALL_KEY {
            self.state.select(Target::All);
            self.load_metrics();
            return;
        }
        let found = self
            .state
            .repos
            .ready()
            .and_then(|repos| repos.iter().find(|r| r.full_name == last).cloned());
        if let Some(repo) = found {
            self.state.select(Target::Repo(repo));
            self.load_metrics();
        }
    }

    /// Switches the UI language and remembers the choice. Every string
    /// is derived per frame, so nothing is refetched.
    fn set_lang(&mut self, lang: Lang) {
        if lang == self.lang {
            return;
        }
        self.lang = lang;
        self.settings.language = Some(lang);
        self.persist_settings();
    }
}

impl eframe::App for DashboardApp {
    fn logic(&mut self, _ctx: &egui::Context, _frame: &mut eframe::Frame) {
        while let Ok(msg) = self.rx.try_recv() {
            let repos_arrived = matches!(msg, Msg::Repos(Ok(_)));
            let excluded_loaded = matches!(msg, Msg::Excluded(Ok(_)));
            let excluded_saved = matches!(msg, Msg::ExcludedSaved(Ok(_)));
            let applied = self.state.apply(msg);
            if repos_arrived {
                self.restore_last_repo();
                if matches!(self.state.repo_overview, Loadable::Idle) {
                    self.load_overview();
                }
            }
            if (excluded_loaded || excluded_saved)
                && let Some(accounts) = self.state.excluded.ready()
            {
                let text = accounts.join("\n");
                // A reload (after any people change) must not wipe edits
                // the user has not saved yet; a save shows the list as
                // the server normalized it.
                if excluded_saved
                    || self.excluded_input.is_empty()
                    || self.excluded_input == self.excluded_shown
                {
                    self.excluded_input = text.clone();
                }
                self.excluded_shown = text;
            }
            if applied.clear_member_form {
                self.member_form = MemberForm::default();
            }
            if applied.clear_team_form {
                self.team_form = TeamForm::default();
            }
            if applied.notice.is_some() {
                self.notice = applied.notice;
            }
            if applied.reload_repos {
                self.load_repos();
            }
            if applied.reload_people {
                self.load_people();
            }
            if applied.reload_metrics {
                self.load_metrics();
            }
            // New data, repos or people change the comparison tables too.
            if applied.reload_metrics || applied.reload_repos {
                self.load_overview();
            }
        }
        self.poll_sync();
    }

    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        egui::Panel::top("top").show(ui, |ui| self.top_bar(ui));
        if self.show_settings {
            egui::Panel::right("settings")
                .resizable(false)
                .default_size(320.0)
                .show(ui, |ui| self.settings_panel(ui));
        }
        if !self.settings.sidebar_collapsed {
            egui::Panel::left("repos")
                .resizable(true)
                .default_size(190.0)
                .show(ui, |ui| self.repo_list(ui));
        }
        egui::CentralPanel::default().show(ui, |ui| {
            egui::ScrollArea::vertical().show(ui, |ui| match self.view {
                View::Overview => self.overview_page(ui),
                View::Dashboard => self.dashboard(ui),
                View::Repos => self.repos_page(ui),
                View::People => self.people_page(ui),
            });
        });
    }
}

// ----- rendering -----

impl DashboardApp {
    fn top_bar(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.horizontal(|ui| {
            let (arrow, hover) = if self.settings.sidebar_collapsed {
                ("⏵", t.expand_sidebar)
            } else {
                ("⏴", t.collapse_sidebar)
            };
            if ui.button(arrow).on_hover_text(hover).clicked() {
                self.settings.sidebar_collapsed = !self.settings.sidebar_collapsed;
                self.persist_settings();
            }
            ui.heading(
                RichText::new("DevPulse")
                    .strong()
                    .color(ui.visuals().hyperlink_color),
            );
            ui.separator();
            ui.selectable_value(&mut self.view, View::Overview, t.view_overview);
            ui.selectable_value(&mut self.view, View::Dashboard, t.view_dashboard);
            ui.selectable_value(&mut self.view, View::Repos, t.view_repos);
            ui.selectable_value(&mut self.view, View::People, t.view_people);
            ui.separator();

            let w = self.state.window;
            if ui.button("⏴").on_hover_text(t.previous_period).clicked() {
                self.set_window(w.shift(-1));
            }
            ui.label(t.from);
            ui.add(egui::TextEdit::singleline(&mut self.from_input).desired_width(64.0));
            ui.label(t.to_exclusive);
            ui.add(egui::TextEdit::singleline(&mut self.to_input).desired_width(64.0));
            if ui.button(t.apply).clicked() {
                match (
                    self.from_input.parse::<Month>(),
                    self.to_input.parse::<Month>(),
                ) {
                    (Ok(from), Ok(to)) if from < to => self.set_window(Window { from, to }),
                    (Ok(_), Ok(_)) => self.notice = Some(Notice::FromBeforeTo),
                    (Err(_), _) => {
                        self.notice = Some(Notice::BadMonth(self.from_input.trim().into()))
                    }
                    (_, Err(_)) => {
                        self.notice = Some(Notice::BadMonth(self.to_input.trim().into()))
                    }
                }
            }
            if ui.button("⏵").on_hover_text(t.next_period).clicked() {
                self.set_window(w.shift(1));
            }
            // Common periods in one click; the label says which one the
            // current period is, or "Custom".
            let now = Month::current();
            let current = Preset::matching(self.state.window, now);
            let mut picked = None;
            egui::ComboBox::from_id_salt("period-preset")
                .selected_text(current.map_or(t.preset_custom, |p| preset_name(p, t)))
                .show_ui(ui, |ui| {
                    for p in Preset::ALL {
                        if ui
                            .selectable_label(current == Some(p), preset_name(p, t))
                            .clicked()
                        {
                            picked = Some(p);
                        }
                    }
                });
            if let Some(p) = picked {
                self.set_window(p.window(now));
            }

            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                let label = if self.show_settings {
                    t.close_settings
                } else {
                    t.settings
                };
                if ui.button(label).clicked() {
                    self.show_settings = !self.show_settings;
                }
                if ui.button(t.refresh).clicked() {
                    self.load_repos();
                    self.load_metrics();
                    self.load_overview();
                }
                if self.state.report.is_loading() || self.state.repos.is_loading() {
                    ui.spinner();
                }
            });
        });
        if let Some(notice) = &self.notice {
            let color = if notice.is_error() {
                ui.visuals().error_fg_color
            } else {
                theme::Tone::Good.color(ui)
            };
            let dismissed = ui
                .horizontal(|ui| {
                    ui.colored_label(color, notice.text(t));
                    ui.small_button("✕").clicked()
                })
                .inner;
            if dismissed {
                self.notice = None;
            }
        }
    }

    fn settings_panel(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.connection);
        ui.label(t.server_url);
        ui.text_edit_singleline(&mut self.url_input);
        ui.add_space(6.0);

        let hint = if self.token.is_some() {
            t.token_in_use_hint
        } else {
            "DEVPULSE_API_TOKEN"
        };
        ui.label(t.api_token);
        ui.add(
            egui::TextEdit::singleline(&mut self.token_input)
                .password(true)
                .hint_text(hint),
        );
        ui.add_space(6.0);

        ui.horizontal(|ui| {
            if ui.button(t.save_and_connect).clicked() {
                self.save_settings();
            }
            if ui.button(t.test).clicked() {
                self.test_connection();
            }
            if self.token.is_some() && ui.button(t.forget_token).clicked() {
                self.forget_token();
            }
        });

        match &self.state.health {
            Loadable::Idle => {}
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Ready(()) => {
                ui.colored_label(theme::Tone::Good.color(ui), t.connected);
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
            }
        }

        ui.add_space(12.0);
        ui.small(t.keychain_note);

        ui.add_space(12.0);
        ui.separator();
        ui.heading(t.language);
        let mut lang = self.lang;
        ui.horizontal(|ui| {
            for l in Lang::ALL {
                ui.radio_value(&mut lang, l, l.native_name());
            }
        });
        self.set_lang(lang);
        if let Some(candidates) = &self.missing_cjk_font {
            ui.colored_label(ui.visuals().warn_fg_color, (t.no_cjk_font)(candidates));
        }
    }

    fn repo_list(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.repositories);
        ui.separator();
        let mut clicked = None;
        match &self.state.repos {
            Loadable::Idle => {
                ui.label(t.connect_in_settings);
            }
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
            }
            Loadable::Ready(repos) if repos.is_empty() => {
                ui.label(t.no_repos);
            }
            Loadable::Ready(repos) => {
                egui::ScrollArea::vertical().show(ui, |ui| {
                    let all = self.state.selected == Some(Target::All);
                    if ui
                        .selectable_label(all, RichText::new(t.all_repos).strong())
                        .clicked()
                    {
                        clicked = Some(Target::All);
                    }
                    // Grouped by owner, so names are short; the rare long one
                    // is truncated, with the full name on hover.
                    ui.style_mut().wrap_mode = Some(egui::TextWrapMode::Truncate);
                    let mut groups: std::collections::BTreeMap<&str, Vec<&Repo>> =
                        std::collections::BTreeMap::new();
                    for repo in repos {
                        groups.entry(repo.owner.as_str()).or_default().push(repo);
                    }
                    for (owner, mut group) in groups {
                        group.sort_by(|a, b| a.name.cmp(&b.name));
                        ui.add_space(4.0);
                        // Each owner folds on its own; egui remembers which.
                        egui::CollapsingHeader::new(RichText::new(owner).strong().weak())
                            .id_salt(("owner", owner))
                            .default_open(true)
                            .show(ui, |ui| {
                                for repo in group {
                                    let selected = self.state.selected_repo() == Some(repo);
                                    let mut text = RichText::new(&repo.name);
                                    if repo.disabled {
                                        text = text.weak().italics();
                                    }
                                    let hover = match &repo.description {
                                        Some(d) if !d.is_empty() => {
                                            format!("{}\n{d}", repo.full_name)
                                        }
                                        _ => repo.full_name.clone(),
                                    };
                                    let resp =
                                        hover_now(ui.selectable_label(selected, text), &hover);
                                    if resp.clicked() {
                                        clicked = Some(Target::Repo(repo.clone()));
                                    }
                                }
                            });
                    }
                });
            }
        }
        if let Some(target) = clicked
            && self.state.select(target.clone())
        {
            self.settings.last_repo = Some(target.key().to_string());
            self.persist_settings();
            self.view = View::Dashboard;
            self.load_metrics();
        }
    }

    /// The Everyone / team / member picker. Returns true when the choice
    /// changed.
    fn scope_picker(&mut self, ui: &mut egui::Ui) -> bool {
        let t = self.lang.texts();
        let before = self.state.scope.clone();
        let mut scope = before.clone();
        let teams = self.state.teams.ready().cloned().unwrap_or_default();
        let members = self.state.members.ready().cloned().unwrap_or_default();
        egui::ComboBox::from_id_salt("scope")
            .selected_text(self.state.scope_label(&before, t))
            .show_ui(ui, |ui| {
                ui.selectable_value(&mut scope, ScopeParam::Everyone, t.everyone);
                if let ScopeParam::Account(a) = &before {
                    ui.selectable_value(&mut scope, before.clone(), a.as_str());
                }
                for team in &teams {
                    ui.selectable_value(
                        &mut scope,
                        ScopeParam::Team(team.id.clone()),
                        (t.team_label)(&team.name),
                    );
                }
                for m in &members {
                    ui.selectable_value(
                        &mut scope,
                        ScopeParam::Member(m.id.clone()),
                        &m.display_name,
                    );
                }
            });
        if scope == before {
            return false;
        }
        self.state.scope = scope;
        true
    }

    fn by_member_section(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.by_member);
        ui.small(t.by_member_caption);
        ui.add_space(4.0);
        let rows = match &self.state.by_member {
            Loadable::Ready(ByMember { rows, .. }) => rows.clone(),
            Loadable::Loading => {
                ui.spinner();
                return;
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
                return;
            }
            Loadable::Idle => return,
        };
        if rows.is_empty() {
            ui.label(t.no_activity);
            return;
        }

        egui::Grid::new("by-member")
            .num_columns(8)
            .striped(true)
            .spacing([18.0, 6.0])
            .show(ui, |ui| {
                for title in [
                    t.col_name,
                    t.col_prs_opened,
                    t.col_merged,
                    t.col_lead_time,
                    t.builds_per_pr,
                    t.col_ci_failures,
                    t.review_wait,
                    "",
                ] {
                    ui.label(RichText::new(title).strong());
                }
                ui.end_row();
                for row in &rows {
                    let r = &row.report;
                    let name = if row.member_id.is_some() {
                        RichText::new(&row.name)
                    } else {
                        RichText::new(&row.name).weak().italics()
                    };
                    ui.label(name).on_hover_text(row.accounts.join(", "));
                    let opened: u64 = r.pr_size_distribution.iter().map(|b| b.count).sum();
                    ui.label(opened.to_string());
                    ui.label(r.pr_lead_time.count.to_string());
                    ui.label(hours(r.pr_lead_time.count, r.pr_lead_time.avg_hours));
                    ui.label(if r.avg_builds_per_pr > 0.0 {
                        format!("{:.1}", r.avg_builds_per_pr)
                    } else {
                        "—".into()
                    });
                    ui.label(if r.build_failure.total > 0 {
                        format!(
                            "{:.0}% ({}/{})",
                            r.build_failure.rate * 100.0,
                            r.build_failure.failed,
                            r.build_failure.total
                        )
                    } else {
                        "—".into()
                    });
                    ui.label(hours(r.review_wait.count, r.review_wait.avg_hours));
                    match &row.member_id {
                        Some(id) => {
                            if ui
                                .small_button(t.show_button)
                                .on_hover_text(t.show_member_hover)
                                .clicked()
                            {
                                self.state.scope = ScopeParam::Member(id.clone());
                                self.load_metrics();
                            }
                        }
                        None => {
                            if ui
                                .small_button(t.map_button)
                                .on_hover_text(t.map_hover)
                                .clicked()
                            {
                                self.map_account(&row.name);
                            }
                        }
                    }
                    ui.end_row();
                }
            });
    }

    fn overview_page(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(format!(
            "{} · {}",
            t.view_overview,
            self.state.window.label()
        ));
        if let Some(ov) = self.state.repo_overview.ready() {
            let prev = crate::state::Window {
                from: ov.previous.from.parse().unwrap_or(self.state.window.from),
                to: ov.previous.to.parse().unwrap_or(self.state.window.from),
            };
            ui.small((t.compared_with)(&prev.label()));
        }
        ui.add_space(8.0);

        ui.heading(t.repo_comparison);
        let repo_rows: Option<Vec<RepoRow>> = match &self.state.repo_overview {
            Loadable::Ready(ov) => Some(ov.rows.clone()),
            other => {
                loadable_status(ui, other, t);
                None
            }
        };
        if let Some(rows) = repo_rows {
            let table: Vec<TableRow> = rows
                .iter()
                .map(|r| TableRow {
                    name: &r.repo,
                    unmapped: false,
                    current: &r.current,
                    previous: &r.previous,
                    monthly: &r.monthly,
                })
                .collect();
            let action = theme::card(ui, |ui| {
                comparison_table(
                    ui,
                    "repo-overview",
                    t.col_repo,
                    t.open_repo_hover,
                    &Column::REPOS,
                    &mut self.repo_sort,
                    &table,
                    t,
                )
            })
            .inner;
            if let Some(RowAction::Open(i)) = action {
                let repo = self
                    .state
                    .repos
                    .ready()
                    .and_then(|rs| rs.iter().find(|r| r.full_name == rows[i].repo).cloned());
                if let Some(repo) = repo {
                    self.open_dashboard(Target::Repo(repo), ScopeParam::Everyone);
                }
            }
        }

        ui.add_space(16.0);
        ui.heading(t.member_comparison);
        let member_rows: Option<Vec<MemberRow>> = match &self.state.member_overview {
            Loadable::Ready(ov) => Some(ov.rows.clone()),
            other => {
                loadable_status(ui, other, t);
                None
            }
        };
        if let Some(rows) = member_rows {
            let table: Vec<TableRow> = rows
                .iter()
                .map(|r| TableRow {
                    name: &r.name,
                    unmapped: r.member_id.is_none(),
                    current: &r.current,
                    previous: &r.previous,
                    monthly: &r.monthly,
                })
                .collect();
            let action = theme::card(ui, |ui| {
                comparison_table(
                    ui,
                    "member-overview",
                    t.col_name,
                    t.open_member_hover,
                    &Column::MEMBERS,
                    &mut self.member_sort,
                    &table,
                    t,
                )
            })
            .inner;
            match action {
                Some(RowAction::Open(i)) => {
                    let scope = match rows[i].member_id.clone() {
                        Some(id) => ScopeParam::Member(id),
                        None => ScopeParam::Account(rows[i].name.clone()),
                    };
                    self.open_dashboard(Target::All, scope);
                }
                Some(RowAction::Map(i)) => self.map_account(&rows[i].name),
                None => {}
            }
        }
    }

    fn people_page(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.people);
        ui.label(t.people_intro);
        ui.add_space(12.0);
        theme::card(ui, |ui| self.members_section(ui));
        ui.add_space(16.0);
        ui.separator();
        theme::card(ui, |ui| self.teams_section(ui));
        ui.add_space(16.0);
        ui.separator();
        theme::card(ui, |ui| self.excluded_section(ui));
    }

    fn members_section(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.members);
        ui.small(t.members_intro);
        ui.add_space(4.0);
        let members = match &self.state.members {
            Loadable::Ready(ms) => ms.clone(),
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
                return;
            }
            _ => {
                ui.spinner();
                return;
            }
        };
        let teams = self.state.teams.ready().cloned().unwrap_or_default();

        if !members.is_empty() {
            egui::Grid::new("members")
                .num_columns(4)
                .striped(true)
                .spacing([18.0, 6.0])
                .show(ui, |ui| {
                    for title in [t.col_name, t.col_accounts, t.col_teams, ""] {
                        ui.label(RichText::new(title).strong());
                    }
                    ui.end_row();
                    for m in &members {
                        ui.label(&m.display_name);
                        ui.label(if m.accounts.is_empty() {
                            "—".to_string()
                        } else {
                            m.accounts.join(", ")
                        });
                        let team_names: Vec<&str> = teams
                            .iter()
                            .filter(|t| m.team_ids.contains(&t.id))
                            .map(|t| t.name.as_str())
                            .collect();
                        ui.label(if team_names.is_empty() {
                            "—".to_string()
                        } else {
                            team_names.join(", ")
                        });
                        ui.horizontal(|ui| {
                            if self.confirm_delete.as_deref() == Some(m.id.as_str()) {
                                ui.colored_label(
                                    ui.visuals().warn_fg_color,
                                    t.confirm_delete_member,
                                );
                                if ui.button(t.delete).clicked() {
                                    self.delete_member(m.id.clone(), m.display_name.clone());
                                }
                                if ui.button(t.cancel).clicked() {
                                    self.confirm_delete = None;
                                }
                                return;
                            }
                            if ui.button(t.edit).clicked() {
                                self.member_form = MemberForm::edit(m);
                            }
                            if ui.button(t.delete_ellipsis).clicked() {
                                self.confirm_delete = Some(m.id.clone());
                            }
                        });
                        ui.end_row();
                    }
                });
            ui.add_space(8.0);
        }

        let editing = self.member_form.id.is_some();
        ui.label(RichText::new(if editing { t.edit_member } else { t.new_member }).strong());
        ui.horizontal(|ui| {
            ui.label(t.col_name);
            ui.add(egui::TextEdit::singleline(&mut self.member_form.name).desired_width(180.0));
            ui.label(t.col_accounts);
            ui.add(
                egui::TextEdit::singleline(&mut self.member_form.accounts)
                    .hint_text("alice, alice-work")
                    .desired_width(260.0),
            );
            if ui.button(t.save).clicked() {
                self.save_member();
            }
            if (editing || self.member_form != MemberForm::default())
                && ui.button(t.cancel).clicked()
            {
                self.member_form = MemberForm::default();
            }
        });
    }

    fn teams_section(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.teams);
        ui.small(t.teams_intro);
        ui.add_space(4.0);
        let (Some(teams), Some(members)) = (
            self.state.teams.ready().cloned(),
            self.state.members.ready().cloned(),
        ) else {
            ui.spinner();
            return;
        };

        if !teams.is_empty() {
            egui::Grid::new("teams")
                .num_columns(3)
                .striped(true)
                .spacing([18.0, 6.0])
                .show(ui, |ui| {
                    for title in [t.col_name, t.col_members, ""] {
                        ui.label(RichText::new(title).strong());
                    }
                    ui.end_row();
                    for team in &teams {
                        ui.label(&team.name);
                        let names: Vec<&str> = members
                            .iter()
                            .filter(|m| team.member_ids.contains(&m.id))
                            .map(|m| m.display_name.as_str())
                            .collect();
                        ui.label(if names.is_empty() {
                            "—".to_string()
                        } else {
                            names.join(", ")
                        });
                        ui.horizontal(|ui| {
                            if self.confirm_delete.as_deref() == Some(team.id.as_str()) {
                                ui.colored_label(ui.visuals().warn_fg_color, t.confirm_delete_team);
                                if ui.button(t.delete).clicked() {
                                    self.delete_team(team.id.clone(), team.name.clone());
                                }
                                if ui.button(t.cancel).clicked() {
                                    self.confirm_delete = None;
                                }
                                return;
                            }
                            if ui.button(t.edit).clicked() {
                                self.team_form = TeamForm::edit(team);
                            }
                            if ui.button(t.delete_ellipsis).clicked() {
                                self.confirm_delete = Some(team.id.clone());
                            }
                        });
                        ui.end_row();
                    }
                });
            ui.add_space(8.0);
        }

        let editing = self.team_form.id.is_some();
        ui.label(RichText::new(if editing { t.edit_team } else { t.new_team }).strong());
        ui.horizontal(|ui| {
            ui.label(t.col_name);
            ui.add(egui::TextEdit::singleline(&mut self.team_form.name).desired_width(180.0));
        });
        if members.is_empty() {
            ui.small(t.add_members_first);
        } else {
            ui.horizontal_wrapped(|ui| {
                for m in &members {
                    let mut on = self.team_form.member_ids.contains(&m.id);
                    if ui.checkbox(&mut on, &m.display_name).changed() {
                        if on {
                            self.team_form.member_ids.insert(m.id.clone());
                        } else {
                            self.team_form.member_ids.remove(&m.id);
                        }
                    }
                }
            });
        }
        ui.horizontal(|ui| {
            if ui.button(t.save_team).clicked() {
                self.save_team();
            }
            if (editing || self.team_form != TeamForm::default()) && ui.button(t.cancel).clicked() {
                self.team_form = TeamForm::default();
            }
        });
    }

    fn excluded_section(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.excluded_accounts);
        ui.small(t.excluded_intro);
        ui.add_space(4.0);
        if let Loadable::Failed(e) = &self.state.excluded {
            ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
            return;
        }
        ui.add(
            egui::TextEdit::multiline(&mut self.excluded_input)
                .desired_rows(4)
                .desired_width(320.0),
        );
        if ui.button(t.save_excluded).clicked() {
            self.save_excluded();
        }
    }

    fn repos_page(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.heading(t.repositories);
        ui.label(t.repos_intro);
        ui.add_space(8.0);

        ui.horizontal(|ui| {
            ui.label(t.add_repo);
            let resp = ui.add(
                egui::TextEdit::singleline(&mut self.add_input)
                    .hint_text("owner/name")
                    .desired_width(260.0),
            );
            let entered = resp.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter));
            if ui.button(t.add).clicked() || entered {
                self.register_repo();
            }
        });
        ui.small(t.add_repo_note);
        ui.add_space(8.0);
        self.sync_status_line(ui);
        ui.add_space(8.0);

        let repos = match &self.state.repos {
            Loadable::Ready(repos) => repos.clone(),
            Loadable::Loading => {
                ui.spinner();
                return;
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
                return;
            }
            Loadable::Idle => {
                ui.label(t.connect_in_settings);
                return;
            }
        };
        if repos.is_empty() {
            ui.label(t.no_repos_short);
            return;
        }

        let can_sync = matches!(self.state.sync, Loadable::Ready(_)) && !self.state.sync_running();
        theme::card(ui, |ui| {
            egui::Grid::new("repo-admin")
                .num_columns(6)
                .striped(true)
                .spacing([18.0, 8.0])
                .show(ui, |ui| {
                    for title in [
                        t.col_repo,
                        t.col_default_branch,
                        t.col_pr_start,
                        t.col_incident_label,
                        t.col_hotfix_label,
                        "",
                    ] {
                        ui.label(RichText::new(title).strong());
                    }
                    ui.end_row();
                    for repo in &repos {
                        self.repo_row(ui, repo, can_sync);
                        ui.end_row();
                    }
                });
        });
    }

    fn repo_row(&mut self, ui: &mut egui::Ui, repo: &Repo, can_sync: bool) {
        let t = self.lang.texts();
        let mut name = RichText::new(&repo.full_name);
        if repo.disabled {
            name = name.weak().italics();
        }
        ui.label(name)
            .on_hover_text(repo.description.as_deref().unwrap_or(""));
        ui.label(if repo.default_branch.is_empty() {
            "—"
        } else {
            &repo.default_branch
        });

        let editing = self
            .editing
            .as_ref()
            .is_some_and(|e| e.full_name == repo.full_name);
        if editing {
            let edit = self.editing.as_mut().expect("editing");
            ui.add(egui::TextEdit::singleline(&mut edit.pr_start).desired_width(70.0));
            ui.add(egui::TextEdit::singleline(&mut edit.incident_label).desired_width(120.0));
            ui.add(egui::TextEdit::singleline(&mut edit.hotfix_label).desired_width(120.0));
            ui.horizontal(|ui| {
                if ui.button(t.save).clicked() {
                    self.save_repo_edit(repo);
                }
                if ui.button(t.cancel).clicked() {
                    self.editing = None;
                }
            });
            return;
        }

        ui.label(repo.pr_start.to_string());
        ui.label(&repo.incident_label);
        ui.label(&repo.hotfix_label);
        ui.horizontal(|ui| {
            if self.confirm_remove.as_deref() == Some(repo.full_name.as_str()) {
                ui.colored_label(ui.visuals().warn_fg_color, t.confirm_remove_repo);
                if ui.button(t.remove).clicked() {
                    self.remove_repo(repo);
                }
                if ui.button(t.cancel).clicked() {
                    self.confirm_remove = None;
                }
                return;
            }
            if ui.button(t.edit).clicked() {
                self.editing = Some(RepoEdit::from_repo(repo));
            }
            if ui
                .add_enabled(can_sync, egui::Button::new(t.sync))
                .on_disabled_hover_text(t.sync_disabled_hover)
                .clicked()
            {
                self.start_sync(repo);
            }
            if ui.button(t.remove_ellipsis).clicked() {
                self.confirm_remove = Some(repo.full_name.clone());
            }
        });
    }

    fn sync_status_line(&self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        match &self.state.sync {
            Loadable::Failed(e) => {
                // The server's own words stay available on hover.
                ui.colored_label(ui.visuals().warn_fg_color, t.sync_unavailable)
                    .on_hover_text(e.describe(t));
            }
            Loadable::Ready(s) if !s.running.is_empty() => {
                ui.horizontal(|ui| {
                    ui.spinner();
                    ui.label((t.syncing)(
                        &s.running,
                        &short_time(s.started_at.as_deref()),
                    ));
                });
            }
            Loadable::Ready(s) if !s.last_repo.is_empty() => {
                let when = short_time(s.last_finished_at.as_deref());
                if s.last_error.is_empty() {
                    ui.label((t.last_sync_ok)(&s.last_repo, &when));
                } else {
                    ui.colored_label(
                        ui.visuals().error_fg_color,
                        (t.last_sync_failed)(&s.last_repo, &when, &s.last_error),
                    );
                }
            }
            _ => {}
        }
    }

    fn dashboard(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        let Some(target) = self.state.selected.clone() else {
            ui.label(t.select_repo);
            return;
        };
        let name = match &target {
            Target::Repo(r) => r.full_name.clone(),
            Target::All => t.all_repos.to_string(),
        };
        let changed = ui
            .horizontal(|ui| {
                ui.heading(format!("{name} · {}", self.state.window.label()));
                ui.separator();
                ui.label(t.show);
                self.scope_picker(ui)
            })
            .inner;
        if changed {
            self.load_metrics();
        }
        if target.repo().is_some_and(|r| r.disabled) {
            ui.colored_label(ui.visuals().warn_fg_color, t.repo_disabled);
        }
        ui.add_space(8.0);

        match &self.state.report {
            Loadable::Idle => {}
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
            }
            Loadable::Ready(report) => {
                kpi_cards(ui, &kpi::kpis(report, self.state.previous_month(), t), t);
                ui.add_space(12.0);
                ui.columns(2, |cols| {
                    theme::card(&mut cols[0], |ui| size_chart(ui, report, t));
                    theme::card(&mut cols[1], |ui| daily_duration_chart(ui, report, t));
                });

                ui.add_space(12.0);
                ui.separator();
                if target == Target::All {
                    ui.heading("DORA");
                    ui.label(t.dora_needs_repo);
                } else if self.state.scope == ScopeParam::Everyone {
                    dora_section(ui, report, self.state.previous_month(), t);
                } else {
                    ui.heading("DORA");
                    ui.label(t.dora_everyone_only);
                }
            }
        }

        if self.state.scope == ScopeParam::Everyone {
            ui.add_space(12.0);
            ui.separator();
            if target == Target::All {
                ui.heading(t.by_member);
                ui.horizontal(|ui| {
                    ui.label(t.breakdown_on_overview);
                    if ui.button(t.go_to_overview).clicked() {
                        self.view = View::Overview;
                    }
                });
            } else {
                theme::card(ui, |ui| self.by_member_section(ui));
            }
        }

        ui.add_space(12.0);
        ui.separator();
        let trend = self.state.window.trend();
        ui.heading((t.trend_heading)(
            &trend.from.to_string(),
            &trend.to.prev().to_string(),
        ));
        // Months picked by dragging across a trend chart, as indices into
        // the trend; applied once the charts are drawn.
        let mut picked = None;
        match &self.state.trend {
            Loadable::Idle => {}
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
            }
            Loadable::Ready(monthly) => {
                ui.small(t.trend_drag_hint);
                ui.columns(2, |cols| {
                    picked = picked.or(theme::card(&mut cols[0], |ui| {
                        failure_trend_chart(ui, monthly, t)
                    })
                    .inner);
                    picked = picked.or(theme::card(&mut cols[1], |ui| {
                        lead_time_trend_chart(ui, monthly, t)
                    })
                    .inner);
                });
                if monthly.months.iter().any(|r| r.dora.is_some()) {
                    ui.add_space(8.0);
                    ui.columns(2, |cols| {
                        picked = picked.or(theme::card(&mut cols[0], |ui| {
                            deploy_trend_chart(ui, monthly, t)
                        })
                        .inner);
                        picked = picked.or(theme::card(&mut cols[1], |ui| {
                            change_failure_trend_chart(ui, monthly, t)
                        })
                        .inner);
                    });
                }
            }
        }
        if let Some((first, last)) = picked {
            self.set_window(Window {
                from: trend.from.add(first as i32),
                to: trend.from.add(last as i32 + 1),
            });
        }
    }
}

fn dora_section(ui: &mut egui::Ui, report: &Report, previous: Option<&Report>, t: &'static Texts) {
    match (&report.dora, kpi::dora_kpis(report, previous, t)) {
        (Some(d), Some(cards)) => {
            ui.heading((t.dora_heading)(&d.default_branch));
            ui.add_space(4.0);
            kpi_cards(ui, &cards, t);
        }
        _ => {
            ui.heading("DORA");
            ui.label((t.dora_branch_unknown)(&report.repo));
        }
    }
}

fn kpi_cards(ui: &mut egui::Ui, cards: &[Kpi], t: &Texts) {
    ui.columns(cards.len(), |cols| {
        for (ui, card) in cols.iter_mut().zip(cards) {
            let tone = card.status.map(status_tone);
            let resp = theme::card(ui, |ui| {
                titled_row(ui, card.title, card.help);
                let value = RichText::new(&card.value).size(32.0).strong();
                let value = match tone {
                    Some(tone) => value.color(tone.color(ui)),
                    None => value,
                };
                let resp = ui.label(value);
                if let Some(status) = card.status {
                    resp.on_hover_text(status_text(status, t));
                }
                ui.small(&card.detail);
                ui.horizontal(|ui| {
                    ui.small(card.target);
                    if let Some(d) = &card.delta {
                        // Neutral colour: whether "up" is good depends on the
                        // metric, and the ideal is shown right beside it.
                        let color = match d.direction {
                            Direction::Up | Direction::Down => ui.visuals().hyperlink_color,
                            Direction::Flat => ui.visuals().weak_text_color(),
                        };
                        ui.small(RichText::new((t.mom)(&d.text)).color(color))
                            .on_hover_text(t.mom_hover);
                    }
                });
            });
            // The status as a bar on the card's left edge.
            if let Some(tone) = tone {
                let r = resp.response.rect;
                let bar = egui::Rect::from_min_max(r.min, egui::pos2(r.min.x + 4.0, r.max.y));
                ui.painter().rect_filled(
                    bar,
                    egui::CornerRadius {
                        nw: 10,
                        sw: 10,
                        ne: 0,
                        se: 0,
                    },
                    tone.color(ui),
                );
            }
        }
    });
}

fn preset_name(p: Preset, t: &Texts) -> &'static str {
    match p {
        Preset::ThisMonth => t.this_month,
        Preset::LastMonth => t.preset_last_month,
        Preset::ThisYear => t.preset_this_year,
        Preset::LastTwelveMonths => t.preset_last_12,
        Preset::LastYear => t.preset_last_year,
    }
}

fn status_tone(status: kpi::Status) -> Tone {
    match status {
        kpi::Status::OnTarget => Tone::Good,
        kpi::Status::Near => Tone::Near,
        kpi::Status::Off => Tone::Bad,
    }
}

fn status_text(status: kpi::Status, t: &Texts) -> &'static str {
    match status {
        kpi::Status::OnTarget => t.status_on_target,
        kpi::Status::Near => t.status_near,
        kpi::Status::Off => t.status_off,
    }
}

/// A chart title followed by an (i) that explains the chart on hover.
fn chart_title(ui: &mut egui::Ui, title: &str, help: &str) {
    titled_row(ui, title, help);
}

/// A bold title followed by an (i). Wrapped, so a long title (most
/// Chinese ones, in a narrow card) breaks onto the next line instead of
/// widening its column past the window edge.
fn titled_row(ui: &mut egui::Ui, title: &str, help: &str) {
    ui.horizontal_wrapped(|ui| {
        ui.label(RichText::new(title).strong());
        info_icon(ui, help);
    });
}

/// A small circled "i" that explains something: hovering shows `help`
/// right away, clicking pins it in a popup until the next click outside.
/// The hover text is shown directly rather than through `on_hover_text`,
/// whose delay and stillness checks left it hidden in practice.
/// Painted rather than typed: egui's bundled fonts have no info glyph
/// (ℹ, ⓘ), and the CJK fallback font may be missing.
fn info_icon(ui: &mut egui::Ui, help: &str) {
    let size = ui.text_style_height(&egui::TextStyle::Body);
    let (rect, resp) = ui.allocate_exact_size(egui::vec2(size, size), Sense::click());
    if ui.is_rect_visible(rect) {
        let color = if resp.hovered() {
            ui.visuals().strong_text_color()
        } else {
            ui.visuals().weak_text_color()
        };
        let painter = ui.painter();
        painter.circle_stroke(rect.center(), size * 0.4, Stroke::new(1.0, color));
        painter.text(
            rect.center(),
            Align2::CENTER_CENTER,
            "i",
            FontId::proportional(size * 0.6),
            color,
        );
    }
    let resp = resp.on_hover_cursor(egui::CursorIcon::PointingHand);
    let pinned = egui::Popup::from_toggle_button_response(&resp)
        .close_behavior(egui::PopupCloseBehavior::CloseOnClickOutside)
        .width(320.0)
        .show(|ui| ui.label(help))
        .is_some();
    if !pinned {
        hover_now(resp, help);
    }
}

/// Shows `text` next to a widget as soon as it is hovered. egui's
/// `on_hover_text` waits for a delay and a still pointer, which in
/// practice left such help hidden; this skips both checks.
fn hover_now(resp: egui::Response, text: &str) -> egui::Response {
    if resp.hovered() {
        resp.show_tooltip_ui(|ui| {
            ui.set_max_width(ui.spacing().tooltip_width);
            ui.label(text);
        });
    }
    resp
}

/// Spinner, error, or nothing, for a value that is not ready.
fn loadable_status<T>(ui: &mut egui::Ui, l: &Loadable<T>, t: &Texts) {
    match l {
        Loadable::Loading => {
            ui.spinner();
        }
        Loadable::Failed(e) => {
            ui.colored_label(ui.visuals().error_fg_color, e.describe(t));
        }
        Loadable::Idle | Loadable::Ready(_) => {}
    }
}

/// One line of an Overview table, whatever it compares.
struct TableRow<'a> {
    name: &'a str,
    /// An account no member claims: shown in italics with Map….
    unmapped: bool,
    current: &'a Summary,
    previous: &'a Summary,
    monthly: &'a [crate::api::MonthSummary],
}

/// What the user did in an Overview table, by row index.
enum RowAction {
    Open(usize),
    Map(usize),
}

/// A sortable comparison table: name, one cell per column (value and
/// change against the previous period), and a sparkline of the sorted
/// column.
#[allow(clippy::too_many_arguments)]
fn comparison_table(
    ui: &mut egui::Ui,
    id: &str,
    name_title: &str,
    open_hover: &str,
    columns: &[Column],
    sort: &mut Sort,
    rows: &[TableRow],
    t: &Texts,
) -> Option<RowAction> {
    if rows.is_empty() {
        ui.label(t.no_overview_rows);
        return None;
    }
    let mut order: Vec<usize> = (0..rows.len()).collect();
    order.sort_by(|&a, &b| sort.compare(rows[a].current, rows[b].current));

    let mut action = None;
    egui::ScrollArea::horizontal().id_salt(id).show(ui, |ui| {
        egui::Grid::new(id)
            .striped(true)
            .spacing([12.0, 8.0])
            .show(ui, |ui| {
                ui.label(RichText::new(name_title).strong());
                hover_now(ui.label(RichText::new(t.col_trend).strong()), t.help_trend);
                for &col in columns {
                    let sorted = sort.column == col;
                    let arrow = match (sorted, sort.worst_first) {
                        (false, _) => "",
                        (true, true) => " ⏷",
                        (true, false) => " ⏶",
                    };
                    let label = RichText::new(format!("{}{arrow}", col.short_title(t))).strong();
                    // Short titles keep the columns narrow; the full title,
                    // what it counts and how sorting works are on hover.
                    let hover = format!("{}\n{}\n\n{}", col.title(t), col.help(t), t.sort_hover);
                    if hover_now(ui.selectable_label(sorted, label), &hover).clicked() {
                        *sort = sort.click(col);
                    }
                }
                ui.end_row();

                for &i in &order {
                    let row = &rows[i];
                    ui.vertical(|ui| {
                        // A fixed width, truncating long names (the full name
                        // is on hover), so rows stay one line tall.
                        ui.set_width(NAME_WIDTH);
                        ui.style_mut().wrap_mode = Some(egui::TextWrapMode::Truncate);
                        // "owner/name" reads as name, with the owner below.
                        let (owner, name) = match row.name.split_once('/') {
                            Some((o, n)) => (Some(o), n),
                            None => (None, row.name),
                        };
                        let mut text = RichText::new(name).strong();
                        if row.unmapped {
                            text = text.italics();
                        }
                        let hover = if row.unmapped {
                            format!("{}\n{}", t.unmapped_hover, open_hover)
                        } else {
                            format!("{}\n{}", row.name, open_hover)
                        };
                        if hover_now(ui.add(egui::Link::new(text)), &hover).clicked() {
                            action = Some(RowAction::Open(i));
                        }
                        if let Some(owner) = owner {
                            ui.small(RichText::new(owner).weak());
                        }
                        if row.unmapped
                            && ui
                                .small_button(t.map_button)
                                .on_hover_text(t.map_hover)
                                .clicked()
                        {
                            action = Some(RowAction::Map(i));
                        }
                    });
                    sparkline(ui, sort.column, row.monthly);
                    for &col in columns {
                        ui.vertical(|ui| {
                            ui.label(RichText::new(col.format(row.current, t)).strong());
                            if let Some(c) = change(col, row.previous, row.current) {
                                change_tag(ui, &c);
                            }
                        });
                    }
                    ui.end_row();
                }
            });
    });
    action
}

/// Width of the Overview's name column.
const NAME_WIDTH: f32 = 150.0;

/// A change against the previous period as a small tinted tag with an
/// arrow, so the direction does not rely on colour alone.
fn change_tag(ui: &mut egui::Ui, c: &crate::overview::Change) {
    let tone = if c.worse {
        Tone::Bad
    } else if c.better {
        Tone::Good
    } else {
        Tone::Neutral
    };
    // `change` writes "±" for a change that shows as zero, so the sign
    // alone says the direction.
    let text = if let Some(rest) = c.text.strip_prefix('+') {
        format!("⏶{rest}")
    } else if let Some(rest) = c.text.strip_prefix('-') {
        format!("⏷{rest}")
    } else {
        c.text.clone()
    };
    egui::Frame::new()
        .fill(tone.tint(ui))
        .corner_radius(egui::CornerRadius::same(4))
        .inner_margin(egui::Margin::symmetric(5, 1))
        .show(ui, |ui| {
            // A tag is one unit; never break it across lines.
            ui.style_mut().wrap_mode = Some(egui::TextWrapMode::Extend);
            ui.small(RichText::new(text).color(tone.color(ui)));
        });
}

/// A tiny line chart of one column over the months, painted with egui
/// shapes so a table of many rows stays cheap. Months without data
/// break the line.
fn sparkline(ui: &mut egui::Ui, column: Column, months: &[crate::api::MonthSummary]) {
    let (rect, _) = ui.allocate_exact_size(egui::vec2(96.0, 24.0), Sense::hover());
    let values: Vec<Option<f64>> = months.iter().map(|m| column.value(&m.summary)).collect();
    let (lo, hi) = values
        .iter()
        .flatten()
        .fold((f64::INFINITY, f64::NEG_INFINITY), |(lo, hi), &v| {
            (lo.min(v), hi.max(v))
        });
    if !lo.is_finite() || !ui.is_rect_visible(rect) {
        return;
    }
    let steps = values.len().saturating_sub(1).max(1) as f32;
    let at = |i: usize, v: f64| {
        let share = if hi > lo {
            ((v - lo) / (hi - lo)) as f32
        } else {
            0.5
        };
        egui::pos2(
            rect.left() + rect.width() * i as f32 / steps,
            rect.bottom() - 2.0 - (rect.height() - 4.0) * share,
        )
    };
    let color = ui.visuals().hyperlink_color;
    let painter = ui.painter();
    for (i, v) in values.iter().enumerate() {
        let Some(v) = *v else { continue };
        match values.get(i + 1).copied().flatten() {
            Some(next) => {
                painter.line_segment([at(i, v), at(i + 1, next)], Stroke::new(1.5, color));
            }
            // A point with no neighbour after it would be invisible.
            None if i == 0 || values[i - 1].is_none() => {
                painter.circle_filled(at(i, v), 1.5, color);
            }
            None => {}
        }
    }
}

/// Month axis labels for trend charts: x is the month index.
fn month_labels(monthly: &MonthlyReport) -> Vec<String> {
    monthly.months.iter().map(|r| r.from.clone()).collect()
}

fn size_chart(ui: &mut egui::Ui, report: &Report, t: &Texts) {
    let c0 = theme::series(ui, 0);
    chart_title(ui, t.pr_size_distribution, t.help_size);
    match kpi::small_pr_share(report) {
        Some(share) => {
            let status = kpi::small_share_status(share);
            ui.small(
                RichText::new((t.small_share)(share * 100.0)).color(status_tone(status).color(ui)),
            )
            .on_hover_text(status_text(status, t))
        }
        None => ui.small(t.no_prs_in_window),
    };
    let labels: Vec<String> = report
        .pr_size_distribution
        .iter()
        .map(|b| b.bucket.clone())
        .collect();
    let bars = report
        .pr_size_distribution
        .iter()
        .enumerate()
        .map(|(i, b)| {
            Bar::new(i as f64, b.count as f64)
                .name(&b.bucket)
                .width(0.6)
        })
        .collect();
    category_plot(ui, "size", EVERY)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.bar_chart(BarChart::new(t.series_prs, bars).color(c0))
        });
}

fn daily_duration_chart(ui: &mut egui::Ui, report: &Report, t: &Texts) {
    let c0 = theme::series(ui, 0);
    chart_title(ui, t.daily_build_duration, t.help_daily);
    ui.small(t.daily_build_caption);
    let labels: Vec<String> = report
        .daily_build_duration
        .iter()
        .map(|d| d.day.clone())
        .collect();
    let bars = report
        .daily_build_duration
        .iter()
        .enumerate()
        .map(|(i, d)| {
            Bar::new(i as f64, d.avg_seconds)
                .name((t.day_bar)(&d.day, d.count))
                .width(0.7)
        })
        .collect();
    category_plot(ui, "daily", SPARSE)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.bar_chart(BarChart::new(t.series_seconds, bars).color(c0))
        });
}

fn failure_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
    let c0 = theme::series(ui, 0);
    chart_title(ui, t.ci_failure_trend, t.help_failure_trend);
    let labels = month_labels(monthly);
    // Months without PR builds have no rate; leave a gap rather than
    // plotting a misleading 0%.
    let points: Vec<[f64; 2]> = monthly
        .months
        .iter()
        .enumerate()
        .filter(|(_, r)| r.build_failure.total > 0)
        .map(|(i, r)| [i as f64, r.build_failure.rate * 100.0])
        .collect();
    let plot = trend_plot(ui, "failure-trend", monthly)
        .label_formatter(value_label(labels.clone(), "%"))
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(
                Line::new(t.series_failure, PlotPoints::from(points))
                    .color(c0)
                    .width(2.0),
            )
        });
    month_brush(ui, &plot, monthly.months.len())
}

fn lead_time_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
    let (c0, c1, c2) = (
        theme::series(ui, 0),
        theme::series(ui, 1),
        theme::series(ui, 2),
    );
    chart_title(ui, t.lead_time_trend, t.help_lead_trend);
    let labels = month_labels(monthly);
    let series = |f: fn(&Report) -> f64| -> Vec<[f64; 2]> {
        monthly
            .months
            .iter()
            .enumerate()
            .filter(|(_, r)| r.pr_lead_time.count > 0)
            .map(|(i, r)| [i as f64, f(r)])
            .collect()
    };
    let avg = series(|r| r.pr_lead_time.avg_hours);
    let p50 = series(|r| r.pr_lead_time.p50_hours);
    let p90 = series(|r| r.pr_lead_time.p90_hours);
    let plot = trend_plot(ui, "lead-trend", monthly)
        .legend(Legend::default())
        .label_formatter(value_label(labels.clone(), "h"))
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            // The median is the headline, as on the card; the mean and p90
            // show the tail.
            p.line(Line::new("p50", PlotPoints::from(p50)).color(c0).width(2.5));
            p.line(
                Line::new(t.series_avg, PlotPoints::from(avg))
                    .color(c1)
                    .width(1.5),
            );
            p.line(Line::new("p90", PlotPoints::from(p90)).color(c2).width(1.5));
        });
    month_brush(ui, &plot, monthly.months.len())
}

fn deploy_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
    let c0 = theme::series(ui, 0);
    chart_title(ui, t.deploys_per_week_trend, t.help_deploy_trend);
    let labels = month_labels(monthly);
    let bars = monthly
        .months
        .iter()
        .enumerate()
        .filter_map(|(i, r)| {
            let d = r.dora.as_ref()?;
            Some(
                Bar::new(i as f64, d.per_week)
                    .name((t.deploy_bar)(&r.from, d.deployments))
                    .width(0.6),
            )
        })
        .collect();
    let plot = trend_plot(ui, "deploy-trend", monthly)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.bar_chart(BarChart::new(t.series_deploys, bars).color(c0))
        });
    month_brush(ui, &plot, monthly.months.len())
}

fn change_failure_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
    let c0 = theme::series(ui, 0);
    chart_title(ui, t.cfr_trend, t.help_cfr_trend);
    let labels = month_labels(monthly);
    // Months without deployments have no rate (null), so they are gaps.
    let points: Vec<[f64; 2]> = monthly
        .months
        .iter()
        .enumerate()
        .filter_map(|(i, r)| {
            let rate = r.dora.as_ref()?.change_failure_rate?;
            Some([i as f64, rate * 100.0])
        })
        .collect();
    let plot = trend_plot(ui, "cfr-trend", monthly)
        .label_formatter(value_label(labels.clone(), "%"))
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(
                Line::new(t.series_cfr, PlotPoints::from(points))
                    .color(c0)
                    .width(2.0),
            )
        });
    month_brush(ui, &plot, monthly.months.len())
}

/// Grafana-style range selection on a trend chart, whose x axis is the
/// month index. Dragging across the chart highlights whole months;
/// releasing returns the first and last selected index. The drag state
/// lives in egui's temporary memory, keyed by the plot, so the app keeps
/// nothing while a drag is in progress.
fn month_brush(ui: &egui::Ui, plot: &PlotResponse<()>, months: usize) -> Option<(usize, usize)> {
    if months == 0 {
        return None;
    }
    let resp = &plot.response;
    let id = resp.id.with("month-brush");
    let index = |pos: egui::Pos2| {
        let x = plot.transform.value_from_position(pos).x.round();
        x.clamp(0.0, (months - 1) as f64) as usize
    };

    if resp.drag_started_by(egui::PointerButton::Primary)
        && let Some(pos) = resp.interact_pointer_pos()
    {
        let i = index(pos);
        ui.data_mut(|d| d.insert_temp(id, (i, i)));
    }
    // (anchor, current): the pointer position can be gone on the frame
    // the drag ends, so the last known month is kept.
    let (anchor, mut current): (usize, usize) = ui.data(|d| d.get_temp(id))?;
    if let Some(pos) = resp.interact_pointer_pos() {
        current = index(pos);
        ui.data_mut(|d| d.insert_temp(id, (anchor, current)));
    }
    let (first, last) = (anchor.min(current), anchor.max(current));

    let frame = *plot.transform.frame();
    let x0 = plot
        .transform
        .position_from_point_x(first as f64 - 0.5)
        .max(frame.left());
    let x1 = plot
        .transform
        .position_from_point_x(last as f64 + 0.5)
        .min(frame.right());
    ui.painter().rect_filled(
        egui::Rect::from_x_y_ranges(x0..=x1, frame.y_range()),
        0.0,
        ui.visuals().selection.bg_fill.gamma_multiply(0.35),
    );

    if resp.drag_stopped() {
        ui.data_mut(|d| d.remove::<(usize, usize)>(id));
        return Some((first, last));
    }
    None
}

/// A fixed (no pan / zoom) plot whose x axis is a category index, with
/// grid marks on whole categories so every label lines up with a bar
/// or point. `steps` are the three grid spacings; egui_plot only labels
/// the finer ones when there is room, so a long axis thins out its
/// labels instead of overlapping them.
fn category_plot(ui: &egui::Ui, id: &str, steps: [f64; 3]) -> Plot<'static> {
    Plot::new(id)
        // egui_plot draws grid lines in the text colour by default, which
        // makes the major lines as dark as the labels. A faded text colour
        // keeps them readable as a background in light and dark themes.
        .grid_color(ui.visuals().text_color().gamma_multiply(GRID_ALPHA))
        .height(200.0)
        .allow_drag(false)
        .allow_zoom(false)
        .allow_scroll(false)
        // Primary-button drags select months (see `month_brush`); the
        // secondary-button box zoom would compete with that.
        .allow_boxed_zoom(false)
        .include_y(0.0)
        // Room for three-digit values (hours, seconds) on the y axis.
        .y_axis_min_width(32.0)
        // Base-5 steps (5, 25, 125) keep a labelled mark in view for
        // ranges like 0..90, where the default base-10 spacing only
        // labels 0 and hides the too-dense 10s.
        .y_grid_spacer(log_grid_spacer(5))
        .x_grid_spacer(uniform_grid_spacer(move |_| steps))
}

/// A trend chart: one category per month of the trend. The x axis always
/// spans every month, not just the months with data, so the four trend
/// charts line up and a month without data shows as a gap.
fn trend_plot(ui: &egui::Ui, id: &str, monthly: &MonthlyReport) -> Plot<'static> {
    let last = monthly.months.len().saturating_sub(1) as f64;
    category_plot(ui, id, SPARSE)
        .include_x(-0.5)
        .include_x(last + 0.5)
}

/// Opacity of the strongest grid lines relative to the text colour.
const GRID_ALPHA: f32 = 0.3;

/// Label every category: few, short labels (the size buckets).
const EVERY: [f64; 3] = [1.0, 1.0, 1.0];
/// Label months or days sparsely when crowded.
const SPARSE: [f64; 3] = [1.0, 3.0, 12.0];

/// "12.3h", or "—" when there is no sample.
fn hours(count: u64, avg: f64) -> String {
    if count == 0 {
        "—".into()
    } else {
        format!("{avg:.1}h")
    }
}

/// Renders an RFC 3339 timestamp from the server as "2026-09-30 03:15 UTC".
fn short_time(ts: Option<&str>) -> String {
    match ts {
        Some(t) if t.len() >= 16 => format!("{} UTC", t[..16].replacen('T', " ", 1)),
        Some(t) => t.to_string(),
        None => "—".into(),
    }
}

/// The hover label of a trend line's data point: the month, the series
/// and the value with its unit. Away from a data point, nothing: the
/// crosshair alone says where the pointer is.
fn value_label(
    months: Vec<String>,
    unit: &'static str,
) -> impl Fn(&egui_plot::HoverPosition<'_>) -> Option<String> {
    move |pos| match pos {
        egui_plot::HoverPosition::NearDataPoint {
            plot_name,
            position,
            ..
        } => {
            let month = index_label(&months, position.x.round());
            Some(format!("{month}\n{plot_name}: {:.1}{unit}", position.y))
        }
        egui_plot::HoverPosition::Elsewhere { .. } => None,
    }
}

/// Labels integer grid marks with the matching category; other marks
/// stay blank so zoomed-in fractional ticks do not repeat labels.
fn index_label(labels: &[String], value: f64) -> String {
    if value.fract() != 0.0 || value < 0.0 {
        return String::new();
    }
    labels.get(value as usize).cloned().unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::Cell;

    #[test]
    fn shortens_server_timestamps() {
        assert_eq!(
            short_time(Some("2026-09-30T03:15:42.123456Z")),
            "2026-09-30 03:15 UTC"
        );
        assert_eq!(short_time(Some("soon")), "soon");
        assert_eq!(short_time(None), "—");
    }

    #[test]
    fn index_labels_only_whole_marks() {
        let labels = vec!["XS".to_string(), "S".to_string()];
        assert_eq!(index_label(&labels, 1.0), "S");
        assert_eq!(index_label(&labels, 0.5), "");
        assert_eq!(index_label(&labels, 5.0), "");
        assert_eq!(index_label(&labels, -1.0), "");
    }

    /// Runs one headless frame of `body` in an 800×400 screen.
    fn frame(
        ctx: &egui::Context,
        t: f64,
        pointer: Option<egui::Pos2>,
        body: &dyn Fn(&mut egui::Ui),
    ) {
        let mut input = egui::RawInput {
            time: Some(t),
            screen_rect: Some(egui::Rect::from_min_size(
                egui::Pos2::ZERO,
                egui::vec2(800.0, 400.0),
            )),
            ..Default::default()
        };
        if let Some(p) = pointer {
            input.events.push(egui::Event::PointerMoved(p));
        }
        let mut out = ctx.run_ui(input, |ui| {
            egui::CentralPanel::default().show(ui, |ui| body(ui));
        });
        out.textures_delta.clear();
    }

    fn tooltip_layers(ctx: &egui::Context) -> usize {
        ctx.memory(|m| {
            m.layer_ids()
                .filter(|l| l.order == egui::Order::Tooltip)
                .count()
        })
    }

    #[test]
    fn long_title_wraps_inside_a_narrow_column() {
        let ctx = egui::Context::default();
        let width = Cell::new(0.0f32);
        let body = |ui: &mut egui::Ui| {
            ui.allocate_ui(egui::vec2(120.0, 200.0), |ui| {
                let r = ui.scope(|ui| titled_row(ui, "每個 PR 的建置次數 builds per PR", "help"));
                width.set(r.response.rect.width());
            });
        };
        frame(&ctx, 0.0, None, &body);
        assert!(width.get() <= 120.0, "title row is {} wide", width.get());
    }

    #[test]
    fn hovering_the_info_icon_shows_help_immediately() {
        let ctx = egui::Context::default();
        let icon = Cell::new(egui::Pos2::ZERO);
        let body = |ui: &mut egui::Ui| {
            ui.horizontal(|ui| {
                let at = ui.cursor().min;
                info_icon(ui, "help");
                icon.set(at + egui::vec2(6.0, 6.0));
            });
        };
        frame(&ctx, 0.0, None, &body);
        assert_eq!(tooltip_layers(&ctx), 0);
        // One frame after the pointer arrives, with no tooltip delay.
        frame(&ctx, 0.01, Some(icon.get()), &body);
        frame(&ctx, 0.02, None, &body);
        assert_eq!(tooltip_layers(&ctx), 1);
    }

    #[test]
    fn arrow_glyphs_are_bundled() {
        let ctx = egui::Context::default();
        frame(&ctx, 0.0, None, &|_| {});
        // egui's bundled fonts have these (in its icon font), unlike
        // ▲▼◀▶, which only render when a CJK fallback font happens to
        // carry them.
        for c in ['⏶', '⏷', '⏴', '⏵'] {
            let ok = ctx.fonts_mut(|f| f.has_glyph(&egui::FontId::proportional(14.0), c));
            assert!(ok, "{c} would render as a box");
        }
    }

    #[test]
    fn trend_hover_label_names_month_series_and_value() {
        let label = value_label(vec!["2026-06".into(), "2026-07".into()], "%");
        let near = egui_plot::HoverPosition::NearDataPoint {
            plot_name: "failure %",
            position: egui_plot::PlotPoint::new(1.0, 9.44),
            index: 1,
        };
        assert_eq!(label(&near).as_deref(), Some("2026-07\nfailure %: 9.4%"));
        let away = egui_plot::HoverPosition::Elsewhere {
            position: egui_plot::PlotPoint::new(0.4, 3.0),
        };
        assert_eq!(label(&away), None);
    }
}
