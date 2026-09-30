//! Dashboard state and the rules for applying API results to it. No
//! egui here: the UI in `app.rs` renders this state and turns clicks
//! into requests, and this module decides what a response changes.

use crate::api::{ApiError, MonthlyReport, Repo, Report};
use crate::month::Month;

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

/// A finished background request. Metric results carry the generation
/// they were issued under, so a slow response for a previous selection
/// cannot overwrite the current one.
#[derive(Debug)]
pub enum Msg {
    Health(Result<(), ApiError>),
    Repos(Result<Vec<Repo>, ApiError>),
    Report(u64, Result<Report, ApiError>),
    Trend(u64, Result<MonthlyReport, ApiError>),
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
            generation: 0,
        }
    }

    /// Marks the metrics as loading for a new selection or window and
    /// returns the generation the new requests must carry.
    pub fn begin_metrics_load(&mut self) -> u64 {
        self.generation += 1;
        self.report = Loadable::Loading;
        self.trend = Loadable::Loading;
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

    pub fn apply(&mut self, msg: Msg) {
        match msg {
            Msg::Health(r) => self.health = r.into(),
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
            Msg::Report(..) | Msg::Trend(..) => {} // stale
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
}
