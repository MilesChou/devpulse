//! Turns API reports into the text the dashboard's KPI cards show.
//! Kept free of egui so the formatting rules are unit-testable.

use crate::api::{HoursSummary, Report};
use crate::i18n::Texts;

/// One KPI card.
#[derive(Debug, Clone, PartialEq)]
pub struct Kpi {
    pub title: &'static str,
    /// What the card measures, shown on hover.
    pub help: &'static str,
    pub value: String,
    pub detail: String,
    /// What to aim for, shown under the value: the ideal from the project
    /// goals (CLAUDE.md) when there is one, else the better direction.
    pub target: &'static str,
    /// Change against the previous month, when a trend is loaded.
    pub delta: Option<Delta>,
}

/// Month-over-month change of a KPI.
#[derive(Debug, Clone, PartialEq)]
pub struct Delta {
    pub text: String,
    pub direction: Direction,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Direction {
    Up,
    Down,
    Flat,
}

/// Builds the KPI cards for `report`. `previous` is the report of the
/// month before, used for the month-over-month delta; pass `None` when
/// the window spans several months or no trend is loaded.
pub fn kpis(report: &Report, previous: Option<&Report>, t: &'static Texts) -> Vec<Kpi> {
    let bf = &report.build_failure;
    let lt = &report.pr_lead_time;
    let rw = &report.review_wait;

    vec![
        Kpi {
            title: t.ci_failure_rate,
            help: t.help_ci_failure_rate,
            value: if bf.total == 0 {
                "—".into()
            } else {
                format!("{:.1}%", bf.rate * 100.0)
            },
            detail: (t.builds_failed)(bf.failed, bf.total),
            target: t.ideal_zero_pct,
            delta: previous
                .filter(|p| p.build_failure.total > 0 && bf.total > 0)
                .map(|p| delta(p.build_failure.rate * 100.0, bf.rate * 100.0, " pp")),
        },
        Kpi {
            title: t.builds_per_pr,
            help: t.help_builds_per_pr,
            value: if report.avg_builds_per_pr == 0.0 {
                "—".into()
            } else {
                format!("{:.1}", report.avg_builds_per_pr)
            },
            detail: t.repush_proxy.into(),
            target: t.ideal_one,
            delta: previous
                .filter(|p| p.avg_builds_per_pr > 0.0 && report.avg_builds_per_pr > 0.0)
                .map(|p| delta(p.avg_builds_per_pr, report.avg_builds_per_pr, "")),
        },
        Kpi {
            title: t.pr_lead_time,
            help: t.help_pr_lead_time,
            value: if lt.count == 0 {
                "—".into()
            } else {
                format!("{:.1}h", lt.avg_hours)
            },
            detail: (t.pr_lead_detail)(lt.p50_hours, lt.p90_hours, lt.count),
            target: t.ideal_24h,
            delta: previous
                .filter(|p| p.pr_lead_time.count > 0 && lt.count > 0)
                .map(|p| delta(p.pr_lead_time.avg_hours, lt.avg_hours, "h")),
        },
        Kpi {
            title: t.review_wait,
            help: t.help_review_wait,
            value: if rw.count == 0 {
                "—".into()
            } else {
                format!("{:.1}h", rw.avg_hours)
            },
            detail: (t.review_wait_detail)(rw.count),
            target: t.lower_is_better,
            delta: previous
                .filter(|p| p.review_wait.count > 0 && rw.count > 0)
                .map(|p| delta(p.review_wait.avg_hours, rw.avg_hours, "h")),
        },
    ]
}

/// Builds the DORA cards, or `None` when the server has no DORA section
/// (default branch unknown). The project goals set no DORA targets, so
/// the cards state the better direction instead of an ideal value.
pub fn dora_kpis(
    report: &Report,
    previous: Option<&Report>,
    t: &'static Texts,
) -> Option<Vec<Kpi>> {
    let d = report.dora.as_ref()?;
    let prev = previous.and_then(|p| p.dora.as_ref());

    let hours_card =
        |title, help, s: &HoursSummary, detail: String, prev: Option<&HoursSummary>| Kpi {
            title,
            help,
            value: if s.count == 0 {
                "—".into()
            } else {
                format!("{:.1}h", s.avg_hours)
            },
            detail,
            target: t.lower_is_better,
            delta: prev
                .filter(|p| p.count > 0 && s.count > 0)
                .map(|p| delta(p.avg_hours, s.avg_hours, "h")),
        };

    Some(vec![
        Kpi {
            title: t.deployment_frequency,
            help: t.help_deployment_frequency,
            value: format!("{:.1}{}", d.per_week, t.per_week),
            detail: (t.deploy_detail)(d.deployments, &d.default_branch, d.deploy_days),
            target: t.higher_is_better,
            delta: prev.map(|p| delta(p.per_week, d.per_week, t.per_week)),
        },
        hours_card(
            t.lead_time_for_changes,
            t.help_lead_time_for_changes,
            &d.lead_time,
            if d.lead_time.count == 0 {
                t.no_deploys_with_data.into()
            } else {
                (t.deploy_percentiles)(
                    d.lead_time.p50_hours,
                    d.lead_time.p90_hours,
                    d.lead_time.count,
                )
            },
            prev.map(|p| &p.lead_time),
        ),
        Kpi {
            title: t.change_failure_rate,
            help: t.help_change_failure_rate,
            value: d
                .change_failure_rate
                .map_or_else(|| "—".into(), |r| format!("{:.1}%", r * 100.0)),
            detail: (t.cfr_detail)(d.reverts, d.hotfixes, &d.hotfix_label),
            target: t.lower_is_better,
            delta: match (
                prev.and_then(|p| p.change_failure_rate),
                d.change_failure_rate,
            ) {
                (Some(p), Some(c)) => Some(delta(p * 100.0, c * 100.0, " pp")),
                _ => None,
            },
        },
        hours_card(
            t.recovery_time,
            t.help_recovery_time,
            &d.recovery,
            (t.recovery_detail)(
                d.recovery_from_reverts,
                d.recovery_from_incidents,
                &d.incident_label,
            ),
            prev.map(|p| &p.recovery),
        ),
    ])
}

/// Formats `current - previous` as "+2.0 pp", "-1.5h" or "±0.0".
/// Changes that round to zero at one decimal count as flat. Plain signs
/// rather than arrows: egui's bundled fonts have no arrow glyphs.
pub fn delta(previous: f64, current: f64, unit: &str) -> Delta {
    let diff = current - previous;
    let rounded = (diff * 10.0).round() / 10.0;
    let (direction, text) = if rounded > 0.0 {
        (Direction::Up, format!("+{rounded:.1}{unit}"))
    } else if rounded < 0.0 {
        (Direction::Down, format!("{rounded:.1}{unit}"))
    } else {
        (Direction::Flat, format!("±0.0{unit}"))
    };
    Delta { text, direction }
}

/// Share of PRs in the two smallest buckets (XS, S), the "skewed toward
/// small PRs" goal. `None` when the window has no PRs.
pub fn small_pr_share(report: &Report) -> Option<f64> {
    let total: u64 = report.pr_size_distribution.iter().map(|b| b.count).sum();
    if total == 0 {
        return None;
    }
    let small: u64 = report
        .pr_size_distribution
        .iter()
        .filter(|b| b.bucket == "XS" || b.bucket == "S")
        .map(|b| b.count)
        .sum();
    Some(small as f64 / total as f64)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::i18n::{EN, ZH_TW};

    const GOLDEN: &str = include_str!("../../internal/http/testdata/metrics.json");
    const GOLDEN_MONTHLY: &str = include_str!("../../internal/http/testdata/metrics_monthly.json");

    fn report() -> Report {
        serde_json::from_str(GOLDEN).unwrap()
    }

    #[test]
    fn formats_golden_report() {
        let cards = kpis(&report(), None, &EN);
        let values: Vec<_> = cards.iter().map(|k| k.value.as_str()).collect();
        assert_eq!(values, ["66.7%", "1.5", "20.0h", "2.0h"]);
        assert_eq!(cards[0].detail, "2 / 3 PR builds failed");
        assert_eq!(cards[2].detail, "p50 20.0h · p90 28.0h · 3 merged PRs");
        assert!(cards.iter().all(|k| k.delta.is_none()));
    }

    #[test]
    fn empty_month_shows_dashes_and_no_delta() {
        let monthly: crate::api::MonthlyReport = serde_json::from_str(GOLDEN_MONTHLY).unwrap();
        let (april, may) = (&monthly.months[0], &monthly.months[1]);

        let cards = kpis(april, None, &EN);
        assert!(cards.iter().all(|k| k.value == "—"), "{cards:?}");

        // April had no data, so May has nothing to compare against.
        assert!(
            kpis(may, Some(april), &EN)
                .iter()
                .all(|k| k.delta.is_none())
        );
    }

    #[test]
    fn delta_directions() {
        assert_eq!(delta(3.0, 5.0, " pp").text, "+2.0 pp");
        assert_eq!(delta(3.0, 5.0, " pp").direction, Direction::Up);
        assert_eq!(delta(20.0, 18.5, "h").text, "-1.5h");
        assert_eq!(delta(20.0, 18.5, "h").direction, Direction::Down);
        assert_eq!(delta(1.0, 1.04, "").direction, Direction::Flat);
        assert_eq!(delta(1.0, 1.04, "").text, "±0.0");
    }

    #[test]
    fn delta_against_previous_month() {
        let cur = report();
        let mut prev = report();
        prev.build_failure.rate = 0.5;
        let cards = kpis(&cur, Some(&prev), &EN);
        assert_eq!(cards[0].delta.as_ref().unwrap().text, "+16.7 pp");
    }

    #[test]
    fn formats_golden_dora() {
        let cards = dora_kpis(&report(), None, &EN).expect("dora cards");
        let values: Vec<_> = cards.iter().map(|k| k.value.as_str()).collect();
        assert_eq!(values, ["2.1/wk", "22.0h", "33.3%", "36.0h"]);
        assert_eq!(cards[0].detail, "3 deploys into main · 3 deploy days");
        assert_eq!(cards[1].detail, "p50 22.0h · p90 30.0h · 3 deploys");
        assert_eq!(cards[2].detail, "1 reverts + 0 hotfixes (label \"hotfix\")");
        assert_eq!(
            cards[3].detail,
            "1 from reverts · 1 incidents (label \"incident\")"
        );
    }

    #[test]
    fn dora_without_deployments_or_section() {
        let monthly: crate::api::MonthlyReport = serde_json::from_str(GOLDEN_MONTHLY).unwrap();
        let (april, may) = (&monthly.months[0], &monthly.months[1]);

        let cards = dora_kpis(april, None, &EN).unwrap();
        assert_eq!(cards[0].value, "0.0/wk");
        assert!(cards[1..].iter().all(|k| k.value == "—"), "{cards:?}");

        // Only deployment frequency can be compared against an empty April.
        let deltas: Vec<_> = dora_kpis(may, Some(april), &EN)
            .unwrap()
            .into_iter()
            .map(|k| k.delta.is_some())
            .collect();
        assert_eq!(deltas, [true, false, false, false]);

        let mut no_branch = report();
        no_branch.dora = None;
        assert!(dora_kpis(&no_branch, None, &EN).is_none());
    }

    #[test]
    fn small_share() {
        // Golden: XS 1, S 1, L 1.
        let share = small_pr_share(&report()).unwrap();
        assert!((share - 2.0 / 3.0).abs() < 1e-9);

        let monthly: crate::api::MonthlyReport = serde_json::from_str(GOLDEN_MONTHLY).unwrap();
        assert_eq!(small_pr_share(&monthly.months[0]), None);
    }

    #[test]
    fn formats_golden_report_in_chinese() {
        let cards = kpis(&report(), None, &ZH_TW);
        let titles: Vec<_> = cards.iter().map(|k| k.title).collect();
        assert_eq!(
            titles,
            [
                "CI 失敗率",
                "每個 PR 的建置次數",
                "PR 前置時間",
                "等待審查時間"
            ]
        );
        // Values are numbers and stay the same in every language.
        let values: Vec<_> = cards.iter().map(|k| k.value.as_str()).collect();
        assert_eq!(values, ["66.7%", "1.5", "20.0h", "2.0h"]);
        assert_eq!(cards[0].detail, "3 次 PR 建置中失敗 2 次");
        assert_eq!(cards[2].detail, "p50 20.0h · p90 28.0h · 3 個已合併 PR");
        assert_eq!(cards[3].target, "越低越好");
    }

    #[test]
    fn formats_golden_dora_in_chinese() {
        let cards = dora_kpis(&report(), None, &ZH_TW).expect("dora cards");
        let values: Vec<_> = cards.iter().map(|k| k.value.as_str()).collect();
        assert_eq!(values, ["2.1/週", "22.0h", "33.3%", "36.0h"]);
        assert_eq!(cards[0].detail, "3 次部署到 main · 3 個部署日");
        assert_eq!(cards[1].detail, "p50 22.0h · p90 30.0h · 3 次部署");
        assert_eq!(
            cards[2].detail,
            "1 次 revert + 0 次 hotfix（標籤「hotfix」）"
        );
        assert_eq!(
            cards[3].detail,
            "1 次來自 revert · 1 次事故（標籤「incident」）"
        );
    }

    #[test]
    fn empty_month_in_chinese() {
        let monthly: crate::api::MonthlyReport = serde_json::from_str(GOLDEN_MONTHLY).unwrap();
        let april = &monthly.months[0];
        assert!(kpis(april, None, &ZH_TW).iter().all(|k| k.value == "—"));
        let dora = dora_kpis(april, None, &ZH_TW).unwrap();
        assert_eq!(dora[0].value, "0.0/週");
        assert_eq!(dora[1].detail, "沒有可計算的部署");
    }
}
