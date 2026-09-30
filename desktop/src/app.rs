//! The egui dashboard: renders `State` and runs API calls on worker
//! threads, which report back over a channel.

use std::path::PathBuf;
use std::sync::mpsc::{Receiver, Sender, channel};
use std::thread;

use eframe::egui::{self, Align2, Color32, FontId, RichText, Sense, Stroke};
use egui_plot::{
    Bar, BarChart, Legend, Line, Plot, PlotPoints, PlotResponse, log_grid_spacer,
    uniform_grid_spacer,
};

use crate::api::{Client, MonthlyReport, Report};
use crate::i18n::{Lang, Texts};
use crate::kpi::{self, Direction, Kpi};
use crate::month::Month;
use crate::settings::{self, SecretStore, Settings};
use crate::state::{Loadable, Msg, State, Window};

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

/// A message under the top bar. Kept as data rather than text so it
/// follows a language switch.
#[derive(Debug, Clone, PartialEq)]
enum Notice {
    KeychainReadFailed(String),
    UrlRequired,
    TokenSaved,
    TokenSaveFailed(String),
    NoToken,
    TokenRemoved,
    TokenRemoveFailed(String),
    SettingsSaveFailed(String),
    FromBeforeTo,
    BadMonth(String),
}

impl Notice {
    fn is_error(&self) -> bool {
        !matches!(self, Self::TokenSaved | Self::TokenRemoved)
    }

    fn text(&self, t: &Texts) -> String {
        match self {
            Self::KeychainReadFailed(e) => (t.keychain_read_failed)(e),
            Self::UrlRequired => t.url_required.into(),
            Self::TokenSaved => t.token_saved.into(),
            Self::TokenSaveFailed(e) => (t.token_save_failed)(e),
            Self::NoToken => t.no_token.into(),
            Self::TokenRemoved => t.token_removed.into(),
            Self::TokenRemoveFailed(e) => (t.token_remove_failed)(e),
            Self::SettingsSaveFailed(e) => (t.settings_save_failed)(e),
            Self::FromBeforeTo => t.from_before_to.into(),
            Self::BadMonth(input) => (t.bad_month)(input),
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
            lang,
            missing_cjk_font,
            show_settings: false,
            token_input: String::new(),
            notice: None,
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
        self.spawn(move || Msg::Repos(client.list_repos()));
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
            self.state.apply(msg);
            if repos_arrived {
                self.restore_last_repo();
            }
        }
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
            egui::ScrollArea::vertical().show(ui, |ui| self.dashboard(ui));
        });
    }
}

// ----- rendering -----

impl DashboardApp {
    fn top_bar(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        ui.horizontal(|ui| {
            ui.heading("DevPulse");
            ui.separator();

            let w = self.state.window;
            if ui.button("◀").on_hover_text(t.previous_period).clicked() {
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
            if ui.button("▶").on_hover_text(t.next_period).clicked() {
                self.set_window(w.shift(1));
            }
            if ui.button(t.this_month).clicked() {
                self.set_window(Window::single(Month::current()));
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
                success_color(ui)
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
                ui.colored_label(success_color(ui), t.connected);
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

    fn dashboard(&mut self, ui: &mut egui::Ui) {
        let t = self.lang.texts();
        let Some(repo) = &self.state.selected else {
            ui.label(t.select_repo);
            return;
        };
        ui.heading(format!(
            "{} · {}",
            repo.full_name,
            self.state.window.label()
        ));
        if repo.disabled {
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
                    size_chart(&mut cols[0], report, t);
                    daily_duration_chart(&mut cols[1], report, t);
                });

                ui.add_space(12.0);
                ui.separator();
                dora_section(ui, report, self.state.previous_month(), t);
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
                    picked = picked.or(failure_trend_chart(&mut cols[0], monthly, t));
                    picked = picked.or(lead_time_trend_chart(&mut cols[1], monthly, t));
                });
                if monthly.months.iter().any(|r| r.dora.is_some()) {
                    ui.add_space(8.0);
                    ui.columns(2, |cols| {
                        picked = picked.or(deploy_trend_chart(&mut cols[0], monthly, t));
                        picked = picked.or(change_failure_trend_chart(&mut cols[1], monthly, t));
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

fn kpi_cards(ui: &mut egui::Ui, cards: &[Kpi], t: &Texts) {
    ui.columns(cards.len(), |cols| {
        for (ui, card) in cols.iter_mut().zip(cards) {
            egui::Frame::group(ui.style()).show(ui, |ui| {
                ui.set_min_width(ui.available_width());
                titled_row(ui, card.title, card.help);
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
                        ui.small(RichText::new((t.mom)(&d.text)).color(color))
                            .on_hover_text(t.mom_hover);
                    }
                });
            });
        }
    });
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
    if !pinned && resp.hovered() {
        resp.show_tooltip_ui(|ui| {
            ui.set_max_width(ui.spacing().tooltip_width);
            ui.label(help);
        });
    }
}

/// Month axis labels for trend charts: x is the month index.
fn month_labels(monthly: &MonthlyReport) -> Vec<String> {
    monthly.months.iter().map(|r| r.from.clone()).collect()
}

fn size_chart(ui: &mut egui::Ui, report: &Report, t: &Texts) {
    chart_title(ui, t.pr_size_distribution, t.help_size);
    match kpi::small_pr_share(report) {
        Some(share) => ui.small((t.small_share)(share * 100.0)),
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
        .show(ui, |p| p.bar_chart(BarChart::new(t.series_prs, bars)));
}

fn daily_duration_chart(ui: &mut egui::Ui, report: &Report, t: &Texts) {
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
        .show(ui, |p| p.bar_chart(BarChart::new(t.series_seconds, bars)));
}

fn failure_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
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
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(Line::new(t.series_failure, PlotPoints::from(points)))
        });
    month_brush(ui, &plot, monthly.months.len())
}

fn lead_time_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
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
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(Line::new(t.series_avg, PlotPoints::from(avg)));
            p.line(Line::new("p50", PlotPoints::from(p50)));
            p.line(Line::new("p90", PlotPoints::from(p90)));
        });
    month_brush(ui, &plot, monthly.months.len())
}

fn deploy_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
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
        .show(ui, |p| p.bar_chart(BarChart::new(t.series_deploys, bars)));
    month_brush(ui, &plot, monthly.months.len())
}

fn change_failure_trend_chart(
    ui: &mut egui::Ui,
    monthly: &MonthlyReport,
    t: &Texts,
) -> Option<(usize, usize)> {
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
        .x_axis_formatter(move |mark, _| index_label(&labels, mark.value))
        .show(ui, |p| {
            p.line(Line::new(t.series_cfr, PlotPoints::from(points)))
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
}
