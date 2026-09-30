//! The egui dashboard: renders `State` and runs API calls on worker
//! threads, which report back over a channel.

use std::path::PathBuf;
use std::sync::mpsc::{Receiver, Sender, channel};
use std::thread;
use std::time::{Duration, Instant};

use eframe::egui::{self, Color32, RichText};
use egui_plot::{
    Bar, BarChart, Legend, Line, Plot, PlotPoints, log_grid_spacer, uniform_grid_spacer,
};

use crate::api::{Client, MonthlyReport, Repo, Report};
use crate::kpi::{self, Direction, Kpi};
use crate::month::Month;
use crate::settings::{self, SecretStore, Settings};
use crate::state::{Loadable, Msg, RepoEdit, State, Window};

/// How often to poll the server while a sync runs.
const SYNC_POLL: Duration = Duration::from_secs(2);

/// The page shown in the central panel.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum View {
    Dashboard,
    Repos,
}

/// Builds the token store for a server URL.
pub type SecretStoreFactory = Box<dyn Fn(&str) -> Box<dyn SecretStore>>;

/// Session-only connection values, from `DEVPULSE_SERVER_URL` and
/// `DEVPULSE_API_TOKEN`. They win over the settings file and keychain
/// but are never written to either, so scripted or CI launches leave no
/// trace in the user's keychain.
#[derive(Debug, Default, Clone)]
pub struct Overrides {
    pub base_url: Option<String>,
    pub token: Option<String>,
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

    // Settings form.
    show_settings: bool,
    url_input: String,
    token_input: String,
    notice: Option<(bool, String)>, // (is_error, text)

    // Window form.
    from_input: String,
    to_input: String,

    // Repos page.
    view: View,
    add_input: String,
    editing: Option<RepoEdit>,
    /// `owner/name` awaiting a second click to confirm removal.
    confirm_remove: Option<String>,
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
    ) -> Self {
        let settings = settings_path
            .as_deref()
            .map(settings::load)
            .unwrap_or_default();
        apply_text_sizes(&ctx);
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
            show_settings: false,
            token_input: String::new(),
            notice: None,
            view: View::Dashboard,
            add_input: String::new(),
            editing: None,
            confirm_remove: None,
            last_sync_poll: None,
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
                app.notice = Some((true, format!("Cannot read the keychain: {e}")));
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
                self.notice = Some((true, e));
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
        let (Some(client), Some(repo)) = (self.client(), self.state.selected.clone()) else {
            return;
        };
        let generation = self.state.begin_metrics_load();
        let window = self.state.window;

        let (c, r) = (client.clone(), repo.clone());
        self.spawn(move || Msg::Report(generation, c.metrics(&r, window.from, window.to)));

        let trend = window.trend();
        self.spawn(move || {
            Msg::Trend(
                generation,
                client.monthly_metrics(&repo, trend.from, trend.to),
            )
        });
    }

    fn set_window(&mut self, window: Window) {
        self.state.window = window;
        self.from_input = window.from.to_string();
        self.to_input = window.to.to_string();
        self.load_metrics();
    }

    /// Saves the URL to the settings file and the token to the keychain,
    /// then reconnects.
    fn save_settings(&mut self) {
        let url = self.url_input.trim().trim_end_matches('/').to_string();
        if url.is_empty() {
            self.notice = Some((true, "Server URL is required.".into()));
            return;
        }
        let url_changed = url != self.base_url().trim_end_matches('/');
        self.settings.base_url = url;
        self.url_override = None;
        self.persist_settings();

        let typed = self.token_input.trim().to_string();
        let mut store = (self.make_store)(&self.settings.base_url);
        if !typed.is_empty() {
            self.notice = match store.set(&typed) {
                Ok(()) => Some((
                    false,
                    "Saved. The token is stored in the OS keychain.".into(),
                )),
                Err(e) => Some((
                    true,
                    format!(
                        "Could not save the token to the keychain ({e}); it is kept for this session only."
                    ),
                )),
            };
            self.token_input.clear();
        }
        self.token = settings::token_after_save(self.token.take(), url_changed, &typed, || {
            store.get().ok().flatten()
        });

        if self.token.is_none() {
            self.notice = Some((true, "No API token for this server.".into()));
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
            Ok(()) => (false, "Token removed from the keychain.".into()),
            Err(e) => (true, format!("Could not remove the token: {e}")),
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
            self.notice = Some((true, format!("Could not save settings: {e}")));
        }
    }

    fn restore_last_repo(&mut self) {
        if self.state.selected.is_some() {
            return;
        }
        let Some(last) = self.settings.last_repo.clone() else {
            return;
        };
        let found = self
            .state
            .repos
            .ready()
            .and_then(|repos| repos.iter().find(|r| r.full_name == last).cloned());
        if let Some(repo) = found {
            self.state.select(repo);
            self.load_metrics();
        }
    }
}

impl eframe::App for DashboardApp {
    fn logic(&mut self, _ctx: &egui::Context, _frame: &mut eframe::Frame) {
        while let Ok(msg) = self.rx.try_recv() {
            let repos_arrived = matches!(msg, Msg::Repos(Ok(_)));
            let applied = self.state.apply(msg);
            if repos_arrived {
                self.restore_last_repo();
            }
            if applied.notice.is_some() {
                self.notice = applied.notice;
            }
            if applied.reload_repos {
                self.load_repos();
            }
            if applied.reload_metrics {
                self.load_metrics();
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
        egui::Panel::left("repos")
            .resizable(true)
            .default_size(220.0)
            .show(ui, |ui| self.repo_list(ui));
        egui::CentralPanel::default().show(ui, |ui| {
            egui::ScrollArea::vertical().show(ui, |ui| match self.view {
                View::Dashboard => self.dashboard(ui),
                View::Repos => self.repos_page(ui),
            });
        });
    }
}

// ----- rendering -----

impl DashboardApp {
    fn top_bar(&mut self, ui: &mut egui::Ui) {
        ui.horizontal(|ui| {
            ui.heading("DevPulse");
            ui.separator();
            ui.selectable_value(&mut self.view, View::Dashboard, "Dashboard");
            ui.selectable_value(&mut self.view, View::Repos, "Repos");
            ui.separator();

            let w = self.state.window;
            if ui.button("◀").on_hover_text("Previous period").clicked() {
                self.set_window(w.shift(-1));
            }
            ui.label("From");
            ui.add(egui::TextEdit::singleline(&mut self.from_input).desired_width(64.0));
            ui.label("to (exclusive)");
            ui.add(egui::TextEdit::singleline(&mut self.to_input).desired_width(64.0));
            if ui.button("Apply").clicked() {
                match (
                    self.from_input.parse::<Month>(),
                    self.to_input.parse::<Month>(),
                ) {
                    (Ok(from), Ok(to)) if from < to => self.set_window(Window { from, to }),
                    (Ok(_), Ok(_)) => self.notice = Some((true, "From must be before To.".into())),
                    (Err(e), _) | (_, Err(e)) => self.notice = Some((true, e)),
                }
            }
            if ui.button("▶").on_hover_text("Next period").clicked() {
                self.set_window(w.shift(1));
            }
            if ui.button("This month").clicked() {
                self.set_window(Window::single(Month::current()));
            }

            ui.with_layout(egui::Layout::right_to_left(egui::Align::Center), |ui| {
                let label = if self.show_settings {
                    "Close settings"
                } else {
                    "Settings"
                };
                if ui.button(label).clicked() {
                    self.show_settings = !self.show_settings;
                }
                if ui.button("Refresh").clicked() {
                    self.load_repos();
                    self.load_metrics();
                }
                if self.state.report.is_loading() || self.state.repos.is_loading() {
                    ui.spinner();
                }
            });
        });
        if let Some((is_error, text)) = &self.notice {
            let color = if *is_error {
                ui.visuals().error_fg_color
            } else {
                success_color(ui)
            };
            let dismissed = ui
                .horizontal(|ui| {
                    ui.colored_label(color, text);
                    ui.small_button("Dismiss").clicked()
                })
                .inner;
            if dismissed {
                self.notice = None;
            }
        }
    }

    fn settings_panel(&mut self, ui: &mut egui::Ui) {
        ui.heading("Connection");
        ui.label("DevPulse server URL (`devpulse serve`)");
        ui.text_edit_singleline(&mut self.url_input);
        ui.add_space(6.0);

        let hint = if self.token.is_some() {
            "(in use; leave empty to keep)"
        } else {
            "DEVPULSE_API_TOKEN"
        };
        ui.label("API token");
        ui.add(
            egui::TextEdit::singleline(&mut self.token_input)
                .password(true)
                .hint_text(hint),
        );
        ui.add_space(6.0);

        ui.horizontal(|ui| {
            if ui.button("Save & connect").clicked() {
                self.save_settings();
            }
            if ui.button("Test").clicked() {
                self.test_connection();
            }
            if self.token.is_some() && ui.button("Forget token").clicked() {
                self.forget_token();
            }
        });

        match &self.state.health {
            Loadable::Idle => {}
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Ready(()) => {
                ui.colored_label(
                    success_color(ui),
                    "Connected: server is up and the token works.",
                );
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e);
            }
        }

        ui.add_space(12.0);
        ui.small(
            "The token is kept in the OS keychain, one entry per server URL. \
             GitHub and CI tokens stay on the server.",
        );
    }

    fn repo_list(&mut self, ui: &mut egui::Ui) {
        ui.heading("Repositories");
        ui.separator();
        let mut clicked = None;
        match &self.state.repos {
            Loadable::Idle => {
                ui.label("Connect to a server in Settings.");
            }
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e);
            }
            Loadable::Ready(repos) if repos.is_empty() => {
                ui.label("No repos yet. Register one with `devpulse repo add <owner/name>`.");
            }
            Loadable::Ready(repos) => {
                egui::ScrollArea::vertical().show(ui, |ui| {
                    for repo in repos {
                        let selected = self.state.selected.as_ref() == Some(repo);
                        let mut text = RichText::new(&repo.full_name);
                        if repo.disabled {
                            text = text.weak().italics();
                        }
                        let resp = ui.selectable_label(selected, text);
                        let resp = match &repo.description {
                            Some(d) if !d.is_empty() => resp.on_hover_text(d),
                            _ => resp,
                        };
                        if resp.clicked() {
                            clicked = Some(repo.clone());
                        }
                    }
                });
            }
        }
        if let Some(repo) = clicked
            && self.state.select(repo.clone())
        {
            self.settings.last_repo = Some(repo.full_name);
            self.persist_settings();
            self.load_metrics();
        }
    }

    fn repos_page(&mut self, ui: &mut egui::Ui) {
        ui.heading("Repositories");
        ui.label("Add, configure, sync or remove the repos this server tracks.");
        ui.add_space(8.0);

        ui.horizontal(|ui| {
            ui.label("Add repo");
            let resp = ui.add(
                egui::TextEdit::singleline(&mut self.add_input)
                    .hint_text("owner/name")
                    .desired_width(260.0),
            );
            let entered = resp.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter));
            if ui.button("Add").clicked() || entered {
                self.register_repo();
            }
        });
        ui.small("GitHub metadata is fetched right away; pull requests and builds arrive with the next sync.");
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
                ui.colored_label(ui.visuals().error_fg_color, e);
                return;
            }
            Loadable::Idle => {
                ui.label("Connect to a server in Settings.");
                return;
            }
        };
        if repos.is_empty() {
            ui.label("No repos yet.");
            return;
        }

        let can_sync = matches!(self.state.sync, Loadable::Ready(_)) && !self.state.sync_running();
        egui::Grid::new("repo-admin")
            .num_columns(6)
            .striped(true)
            .spacing([18.0, 8.0])
            .show(ui, |ui| {
                for title in [
                    "Repo",
                    "Default branch",
                    "PR start",
                    "Incident label",
                    "Hotfix label",
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
    }

    fn repo_row(&mut self, ui: &mut egui::Ui, repo: &Repo, can_sync: bool) {
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
                if ui.button("Save").clicked() {
                    self.save_repo_edit(repo);
                }
                if ui.button("Cancel").clicked() {
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
                ui.colored_label(
                    ui.visuals().warn_fg_color,
                    "Delete it and all its synced data?",
                );
                if ui.button("Remove").clicked() {
                    self.remove_repo(repo);
                }
                if ui.button("Cancel").clicked() {
                    self.confirm_remove = None;
                }
                return;
            }
            if ui.button("Edit").clicked() {
                self.editing = Some(RepoEdit::from_repo(repo));
            }
            if ui
                .add_enabled(can_sync, egui::Button::new("Sync"))
                .on_disabled_hover_text("A sync is running, or the server cannot sync")
                .clicked()
            {
                self.start_sync(repo);
            }
            if ui.button("Remove…").clicked() {
                self.confirm_remove = Some(repo.full_name.clone());
            }
        });
    }

    fn sync_status_line(&self, ui: &mut egui::Ui) {
        match &self.state.sync {
            Loadable::Failed(msg) => {
                ui.colored_label(
                    ui.visuals().warn_fg_color,
                    format!(
                        "{}. Run `devpulse sync` on the server instead.",
                        capitalize(msg)
                    ),
                );
            }
            Loadable::Ready(s) if !s.running.is_empty() => {
                ui.horizontal(|ui| {
                    ui.spinner();
                    ui.label(format!(
                        "Syncing {} (started {})…",
                        s.running,
                        short_time(s.started_at.as_deref())
                    ));
                });
            }
            Loadable::Ready(s) if !s.last_repo.is_empty() => {
                let when = short_time(s.last_finished_at.as_deref());
                if s.last_error.is_empty() {
                    ui.label(format!("Last sync: {} finished {when}.", s.last_repo));
                } else {
                    ui.colored_label(
                        ui.visuals().error_fg_color,
                        format!("Last sync: {} failed {when}: {}", s.last_repo, s.last_error),
                    );
                }
            }
            _ => {}
        }
    }

    fn dashboard(&self, ui: &mut egui::Ui) {
        let Some(repo) = &self.state.selected else {
            ui.label("Select a repository.");
            return;
        };
        ui.heading(format!(
            "{} · {}",
            repo.full_name,
            self.state.window.label()
        ));
        if repo.disabled {
            ui.colored_label(
                ui.visuals().warn_fg_color,
                "This repo is disabled upstream; `devpulse sync` skips it.",
            );
        }
        ui.add_space(8.0);

        match &self.state.report {
            Loadable::Idle => {}
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e);
            }
            Loadable::Ready(report) => {
                kpi_cards(ui, &kpi::kpis(report, self.state.previous_month()));
                ui.add_space(12.0);
                ui.columns(2, |cols| {
                    size_chart(&mut cols[0], report);
                    daily_duration_chart(&mut cols[1], report);
                });

                ui.add_space(12.0);
                ui.separator();
                dora_section(ui, report, self.state.previous_month());
            }
        }

        ui.add_space(12.0);
        ui.separator();
        let trend = self.state.window.trend();
        ui.heading(format!("Trend · {} ~ {}", trend.from, trend.to.prev()));
        match &self.state.trend {
            Loadable::Idle => {}
            Loadable::Loading => {
                ui.spinner();
            }
            Loadable::Failed(e) => {
                ui.colored_label(ui.visuals().error_fg_color, e);
            }
            Loadable::Ready(monthly) => {
                ui.columns(2, |cols| {
                    failure_trend_chart(&mut cols[0], monthly);
                    lead_time_trend_chart(&mut cols[1], monthly);
                });
                if monthly.months.iter().any(|r| r.dora.is_some()) {
                    ui.add_space(8.0);
                    ui.columns(2, |cols| {
                        deploy_trend_chart(&mut cols[0], monthly);
                        change_failure_trend_chart(&mut cols[1], monthly);
                    });
                }
            }
        }
    }
}

fn dora_section(ui: &mut egui::Ui, report: &Report, previous: Option<&Report>) {
    match (&report.dora, kpi::dora_kpis(report, previous)) {
        (Some(d), Some(cards)) => {
            ui.heading(format!(
                "DORA · deployment = PR merged into {}",
                d.default_branch
            ));
            ui.add_space(4.0);
            kpi_cards(ui, &cards);
        }
        _ => {
            ui.heading("DORA");
            ui.label(format!(
                "Default branch unknown; run `devpulse repo refresh {}` on the server.",
                report.repo
            ));
        }
    }
}

/// Font sizes, in points. egui's defaults (small 9, body 13, heading 18)
/// are too small to read card details and chart captions comfortably on
/// a dashboard. Cmd/Ctrl + and - still zoom the whole UI on top of this.
fn apply_text_sizes(ctx: &egui::Context) {
    use egui::{FontFamily, FontId, TextStyle};
    // Both the light and the dark style, so switching the OS theme keeps
    // the sizes.
    ctx.all_styles_mut(|style| {
        style.text_styles = [
            (
                TextStyle::Small,
                FontId::new(12.0, FontFamily::Proportional),
            ),
            (TextStyle::Body, FontId::new(15.0, FontFamily::Proportional)),
            (
                TextStyle::Button,
                FontId::new(15.0, FontFamily::Proportional),
            ),
            (
                TextStyle::Heading,
                FontId::new(22.0, FontFamily::Proportional),
            ),
            (
                TextStyle::Monospace,
                FontId::new(14.0, FontFamily::Monospace),
            ),
        ]
        .into();
    });
}

/// Green for confirmations. egui's visuals have error and warning
/// colours but no success colour, so pick one readable on the current
/// light or dark background.
fn success_color(ui: &egui::Ui) -> Color32 {
    if ui.visuals().dark_mode {
        Color32::LIGHT_GREEN
    } else {
        Color32::DARK_GREEN
    }
}

fn kpi_cards(ui: &mut egui::Ui, cards: &[Kpi]) {
    ui.columns(cards.len(), |cols| {
        for (ui, card) in cols.iter_mut().zip(cards) {
            egui::Frame::group(ui.style()).show(ui, |ui| {
                ui.set_min_width(ui.available_width());
                ui.label(RichText::new(card.title).strong());
                ui.label(RichText::new(&card.value).size(32.0));
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
                        ui.small(RichText::new(format!("{} MoM", d.text)).color(color))
                            .on_hover_text("change vs the previous month");
                    }
                });
            });
        }
    });
}

/// Month axis labels for trend charts: x is the month index.
fn month_labels(monthly: &MonthlyReport) -> Vec<String> {
    monthly.months.iter().map(|r| r.from.clone()).collect()
}

fn size_chart(ui: &mut egui::Ui, report: &Report) {
    ui.label(RichText::new("PR size distribution").strong());
    match kpi::small_pr_share(report) {
        Some(share) => ui.small(format!(
            "{:.0}% small (XS + S) · ideal: mostly small",
            share * 100.0
        )),
        None => ui.small("no PRs in this window"),
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
    category_plot("size", EVERY)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| p.bar_chart(BarChart::new("PRs", bars)));
}

fn daily_duration_chart(ui: &mut egui::Ui, report: &Report) {
    ui.label(RichText::new("Daily build duration").strong());
    ui.small("average seconds per UTC day");
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
                .name(format!("{} ({} builds)", d.day, d.count))
                .width(0.7)
        })
        .collect();
    category_plot("daily", SPARSE)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| p.bar_chart(BarChart::new("seconds", bars)));
}

fn failure_trend_chart(ui: &mut egui::Ui, monthly: &MonthlyReport) {
    ui.label(RichText::new("CI failure rate (%)").strong());
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
    category_plot("failure-trend", SPARSE)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(Line::new("failure %", PlotPoints::from(points)))
        });
}

fn lead_time_trend_chart(ui: &mut egui::Ui, monthly: &MonthlyReport) {
    ui.label(RichText::new("PR lead time (hours)").strong());
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
    category_plot("lead-trend", SPARSE)
        .legend(Legend::default())
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(Line::new("avg", PlotPoints::from(avg)));
            p.line(Line::new("p50", PlotPoints::from(p50)));
            p.line(Line::new("p90", PlotPoints::from(p90)));
        });
}

fn deploy_trend_chart(ui: &mut egui::Ui, monthly: &MonthlyReport) {
    ui.label(RichText::new("Deployments per week").strong());
    let labels = month_labels(monthly);
    let bars = monthly
        .months
        .iter()
        .enumerate()
        .filter_map(|(i, r)| {
            let d = r.dora.as_ref()?;
            Some(
                Bar::new(i as f64, d.per_week)
                    .name(format!("{} ({} deploys)", r.from, d.deployments))
                    .width(0.6),
            )
        })
        .collect();
    category_plot("deploy-trend", SPARSE)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| p.bar_chart(BarChart::new("deploys / week", bars)));
}

fn change_failure_trend_chart(ui: &mut egui::Ui, monthly: &MonthlyReport) {
    ui.label(RichText::new("Change failure rate (%)").strong());
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
    category_plot("cfr-trend", SPARSE)
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(Line::new("change failure %", PlotPoints::from(points)))
        });
}

/// A fixed (no pan / zoom) plot whose x axis is a category index, with
/// grid marks on whole categories so every label lines up with a bar
/// or point. `steps` are the three grid spacings; egui_plot only labels
/// the finer ones when there is room, so a long axis thins out its
/// labels instead of overlapping them.
fn category_plot(id: &str, steps: [f64; 3]) -> Plot<'static> {
    Plot::new(id)
        .height(200.0)
        .allow_drag(false)
        .allow_zoom(false)
        .allow_scroll(false)
        .include_y(0.0)
        // Room for three-digit values (hours, seconds) on the y axis.
        .y_axis_min_width(32.0)
        // Base-5 steps (5, 25, 125) keep a labelled mark in view for
        // ranges like 0..90, where the default base-10 spacing only
        // labels 0 and hides the too-dense 10s.
        .y_grid_spacer(log_grid_spacer(5))
        .x_grid_spacer(uniform_grid_spacer(move |_| steps))
}

/// Label every category: few, short labels (the size buckets).
const EVERY: [f64; 3] = [1.0, 1.0, 1.0];
/// Label months or days sparsely when crowded.
const SPARSE: [f64; 3] = [1.0, 3.0, 12.0];

/// Upper-cases the first letter of a server message ("sync is
/// unavailable" reads as a sentence on its own line).
fn capitalize(s: &str) -> String {
    let mut chars = s.chars();
    match chars.next() {
        Some(c) => c.to_uppercase().chain(chars).collect(),
        None => String::new(),
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

    #[test]
    fn capitalizes_first_letter() {
        assert_eq!(capitalize("sync is unavailable"), "Sync is unavailable");
        assert_eq!(capitalize(""), "");
        assert_eq!(capitalize("Already"), "Already");
    }

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
}
