//! Rules of the Overview tables: which columns exist, which direction
//! is better for each, how rows sort, and when a change against the
//! previous period is worth colouring. Kept free of egui so the rules
//! are unit-testable.

use std::cmp::Ordering;

use crate::api::Summary;
use crate::i18n::Texts;

/// Which way a metric should move.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Better {
    Lower,
    Higher,
    /// A count: more is neither better nor worse.
    Neither,
}

/// A column of an Overview table.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Column {
    PrsOpened,
    PrsMerged,
    LeadTime,
    BuildsPerPr,
    CiFailureRate,
    BuildTime,
    ReviewWait,
    DeploysPerWeek,
}

/// A change within this many percentage points (rates) or this share
/// (everything else) is noise and is not coloured.
const RATE_FLOOR_PP: f64 = 2.0;
const RELATIVE_FLOOR: f64 = 0.10;

impl Column {
    /// Columns of the repo table, in display order.
    pub const REPOS: [Column; 8] = [
        Column::PrsOpened,
        Column::PrsMerged,
        Column::LeadTime,
        Column::BuildsPerPr,
        Column::CiFailureRate,
        Column::BuildTime,
        Column::ReviewWait,
        Column::DeploysPerWeek,
    ];
    /// Columns of the member table: DORA is per repo, so no deploys.
    pub const MEMBERS: [Column; 7] = [
        Column::PrsOpened,
        Column::PrsMerged,
        Column::LeadTime,
        Column::BuildsPerPr,
        Column::CiFailureRate,
        Column::BuildTime,
        Column::ReviewWait,
    ];

    pub fn better(self) -> Better {
        match self {
            Self::PrsOpened | Self::PrsMerged => Better::Neither,
            Self::DeploysPerWeek => Better::Higher,
            _ => Better::Lower,
        }
    }

    pub fn value(self, s: &Summary) -> Option<f64> {
        match self {
            Self::PrsOpened => Some(s.prs_opened as f64),
            Self::PrsMerged => Some(s.prs_merged as f64),
            Self::LeadTime => s.lead_time_p50_hours,
            Self::BuildsPerPr => s.builds_per_pr,
            Self::CiFailureRate => s.ci_failure_rate.map(|r| r * 100.0),
            Self::BuildTime => s.avg_build_seconds,
            Self::ReviewWait => s.review_wait_hours,
            Self::DeploysPerWeek => s.deploys_per_week,
        }
    }

    /// The cell text; "—" without data.
    pub fn format(self, s: &Summary, t: &Texts) -> String {
        let Some(v) = self.value(s) else {
            return "—".into();
        };
        match self {
            Self::PrsOpened | Self::PrsMerged => format!("{v:.0}"),
            Self::LeadTime | Self::ReviewWait => format!("{v:.1}h"),
            Self::BuildsPerPr => format!("{v:.1}"),
            Self::CiFailureRate => format!("{v:.1}%"),
            Self::BuildTime => format_seconds(v),
            Self::DeploysPerWeek => format!("{v:.1}{}", t.per_week),
        }
    }

    pub fn title(self, t: &Texts) -> &'static str {
        match self {
            Self::PrsOpened => t.col_prs_opened,
            Self::PrsMerged => t.col_merged,
            Self::LeadTime => t.col_lead_time,
            Self::BuildsPerPr => t.builds_per_pr,
            Self::CiFailureRate => t.ci_failure_rate,
            Self::BuildTime => t.col_build_time,
            Self::ReviewWait => t.review_wait,
            Self::DeploysPerWeek => t.deployment_frequency,
        }
    }

    /// A short title that fits a narrow table column.
    pub fn short_title(self, t: &Texts) -> &'static str {
        match self {
            Self::PrsOpened => t.short_prs_opened,
            Self::PrsMerged => t.short_prs_merged,
            Self::LeadTime => t.short_lead_time,
            Self::BuildsPerPr => t.short_builds_per_pr,
            Self::CiFailureRate => t.short_ci_failure,
            Self::BuildTime => t.short_build_time,
            Self::ReviewWait => t.short_review_wait,
            Self::DeploysPerWeek => t.short_deploys,
        }
    }

    pub fn help(self, t: &Texts) -> &'static str {
        match self {
            Self::PrsOpened => t.help_prs_opened,
            Self::PrsMerged => t.help_prs_merged,
            Self::LeadTime => t.help_pr_lead_time,
            Self::BuildsPerPr => t.help_builds_per_pr,
            Self::CiFailureRate => t.help_ci_failure_rate,
            Self::BuildTime => t.help_build_time,
            Self::ReviewWait => t.help_review_wait,
            Self::DeploysPerWeek => t.help_deployment_frequency,
        }
    }
}

/// "95s", "6.1m", "1.2h".
pub fn format_seconds(v: f64) -> String {
    if v < 90.0 {
        format!("{v:.0}s")
    } else if v < 5400.0 {
        format!("{:.1}m", v / 60.0)
    } else {
        format!("{:.1}h", v / 3600.0)
    }
}

/// A change against the previous period.
#[derive(Debug, Clone, PartialEq)]
pub struct Change {
    pub text: String,
    /// Worse by more than the noise floor.
    pub worse: bool,
    /// Better by more than the noise floor.
    pub better: bool,
}

/// The change of `column` from `previous` to `current`, or `None` when
/// either period has no data (or the previous value is 0, where a
/// relative change means nothing).
pub fn change(column: Column, previous: &Summary, current: &Summary) -> Option<Change> {
    let (p, c) = (column.value(previous)?, column.value(current)?);
    if column == Column::CiFailureRate {
        let diff = c - p;
        return Some(Change {
            text: signed(diff, 1, " pp"),
            worse: diff > RATE_FLOOR_PP,
            better: diff < -RATE_FLOOR_PP,
        });
    }
    if p == 0.0 {
        return None;
    }
    let rel = (c - p) / p;
    let (worse, better) = match column.better() {
        Better::Lower => (rel > RELATIVE_FLOOR, rel < -RELATIVE_FLOOR),
        Better::Higher => (rel < -RELATIVE_FLOOR, rel > RELATIVE_FLOOR),
        Better::Neither => (false, false),
    };
    Some(Change {
        text: signed(rel * 100.0, 0, "%"),
        worse,
        better,
    })
}

/// "+2.4 pp", "-59%", or "±0%" when the value rounds to zero at the
/// shown precision. Formatting alone would print "-0%" for -0.004, so a
/// change that shows as zero would still point down.
fn signed(v: f64, decimals: usize, unit: &str) -> String {
    let scale = 10f64.powi(decimals as i32);
    let rounded = (v * scale).round() / scale;
    if rounded == 0.0 {
        format!("±{:.*}{unit}", decimals, 0.0)
    } else {
        format!("{rounded:+.decimals$}{unit}")
    }
}

/// How a table is sorted. `worst_first` is the default direction: the
/// rows that most need attention on top.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Sort {
    pub column: Column,
    pub worst_first: bool,
}

impl Sort {
    pub fn new(column: Column) -> Self {
        Self {
            column,
            worst_first: true,
        }
    }

    /// A header click: a new column sorts worst first, the same column
    /// flips the direction.
    pub fn click(self, column: Column) -> Self {
        if column == self.column {
            Self {
                column,
                worst_first: !self.worst_first,
            }
        } else {
            Self::new(column)
        }
    }

    /// Orders rows by their current value. Rows without data always go
    /// last, whichever the direction.
    pub fn compare(self, a: &Summary, b: &Summary) -> Ordering {
        match (self.column.value(a), self.column.value(b)) {
            (None, None) => Ordering::Equal,
            (None, Some(_)) => Ordering::Greater,
            (Some(_), None) => Ordering::Less,
            (Some(x), Some(y)) => {
                // Worst first: highest first for Lower-is-better and for
                // counts (biggest first), lowest first for Higher.
                let worst_first = match self.column.better() {
                    Better::Higher => x.total_cmp(&y),
                    Better::Lower | Better::Neither => y.total_cmp(&x),
                };
                if self.worst_first {
                    worst_first
                } else {
                    worst_first.reverse()
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::i18n::{EN, ZH_TW};

    fn s(f: impl FnOnce(&mut Summary)) -> Summary {
        let mut s = Summary::default();
        f(&mut s);
        s
    }

    #[test]
    fn directions() {
        assert_eq!(Column::CiFailureRate.better(), Better::Lower);
        assert_eq!(Column::BuildTime.better(), Better::Lower);
        assert_eq!(Column::DeploysPerWeek.better(), Better::Higher);
        assert_eq!(Column::PrsOpened.better(), Better::Neither);
        assert!(!Column::MEMBERS.contains(&Column::DeploysPerWeek));
    }

    #[test]
    fn build_time_up_50_percent_is_worse() {
        // The spec example: 120 s → 180 s.
        let c = change(
            Column::BuildTime,
            &s(|s| s.avg_build_seconds = Some(120.0)),
            &s(|s| s.avg_build_seconds = Some(180.0)),
        )
        .unwrap();
        assert_eq!(c.text, "+50%");
        assert!(c.worse);
    }

    #[test]
    fn noise_floor() {
        let rate = |r| s(|s| s.ci_failure_rate = Some(r));
        // +1.5 pp: within the floor.
        assert!(
            !change(Column::CiFailureRate, &rate(0.10), &rate(0.115))
                .unwrap()
                .worse
        );
        // +3 pp: worse.
        let c = change(Column::CiFailureRate, &rate(0.10), &rate(0.13)).unwrap();
        assert!(c.worse);
        assert_eq!(c.text, "+3.0 pp");

        let lead = |h| s(|s| s.lead_time_p50_hours = Some(h));
        assert!(
            !change(Column::LeadTime, &lead(20.0), &lead(21.0))
                .unwrap()
                .worse
        );
        assert!(
            change(Column::LeadTime, &lead(20.0), &lead(23.0))
                .unwrap()
                .worse
        );
        // Getting faster is better, not worse; within the floor is neither.
        let faster = change(Column::LeadTime, &lead(120.0), &lead(60.0)).unwrap();
        assert!(faster.better && !faster.worse);
        assert_eq!(faster.text, "-50%");
        let same = change(Column::LeadTime, &lead(20.0), &lead(21.0)).unwrap();
        assert!(!same.better && !same.worse);
        assert!(
            change(Column::CiFailureRate, &rate(0.13), &rate(0.10))
                .unwrap()
                .better
        );

        let deploys = |d| s(|s| s.deploys_per_week = Some(d));
        assert!(
            change(Column::DeploysPerWeek, &deploys(5.0), &deploys(4.0))
                .unwrap()
                .worse
        );
        assert!(
            !change(Column::DeploysPerWeek, &deploys(5.0), &deploys(8.0))
                .unwrap()
                .worse
        );

        let prs = |n| s(|s| s.prs_opened = n);
        assert!(!change(Column::PrsOpened, &prs(10), &prs(2)).unwrap().worse);
    }

    #[test]
    fn changes_that_show_as_zero_have_no_sign() {
        let lead = |h| s(|s| s.lead_time_p50_hours = Some(h));
        // -0.4 % and +0.4 % both show as zero.
        assert_eq!(
            change(Column::LeadTime, &lead(100.0), &lead(99.6))
                .unwrap()
                .text,
            "±0%"
        );
        assert_eq!(
            change(Column::LeadTime, &lead(99.6), &lead(100.0))
                .unwrap()
                .text,
            "±0%"
        );
        let rate = |r| s(|s| s.ci_failure_rate = Some(r));
        assert_eq!(
            change(Column::CiFailureRate, &rate(0.1004), &rate(0.1))
                .unwrap()
                .text,
            "±0.0 pp"
        );
        assert_eq!(
            change(Column::CiFailureRate, &rate(0.10), &rate(0.13))
                .unwrap()
                .text,
            "+3.0 pp"
        );
    }

    #[test]
    fn no_change_without_data() {
        let none = Summary::default();
        let some = s(|s| s.lead_time_p50_hours = Some(5.0));
        assert_eq!(change(Column::LeadTime, &none, &some), None);
        assert_eq!(change(Column::LeadTime, &some, &none), None);
        // A relative change from 0 means nothing.
        assert_eq!(
            change(Column::PrsOpened, &none, &s(|s| s.prs_opened = 3)),
            None
        );
    }

    #[test]
    fn sorts_worst_first_with_missing_last() {
        let rows = [
            s(|s| s.avg_build_seconds = Some(60.0)),
            Summary::default(),
            s(|s| s.avg_build_seconds = Some(300.0)),
        ];
        let order = |sort: Sort| {
            let mut idx: Vec<usize> = (0..rows.len()).collect();
            idx.sort_by(|&a, &b| sort.compare(&rows[a], &rows[b]));
            idx
        };
        let sort = Sort::new(Column::BuildTime);
        assert_eq!(order(sort), [2, 0, 1], "slowest first, no data last");
        let flipped = sort.click(Column::BuildTime);
        assert!(!flipped.worst_first);
        assert_eq!(order(flipped), [0, 2, 1], "no data stays last");
        assert!(
            flipped.click(Column::LeadTime).worst_first,
            "new column starts worst first"
        );

        // Higher is better: the fewest deploys are the worst.
        let d = [
            s(|s| s.deploys_per_week = Some(5.0)),
            s(|s| s.deploys_per_week = Some(1.0)),
        ];
        assert_eq!(
            Sort::new(Column::DeploysPerWeek).compare(&d[0], &d[1]),
            Ordering::Greater
        );
    }

    #[test]
    fn cell_text() {
        let sum = s(|s| {
            s.ci_failure_rate = Some(0.125);
            s.avg_build_seconds = Some(366.0);
            s.deploys_per_week = Some(2.5);
        });
        assert_eq!(Column::CiFailureRate.format(&sum, &EN), "12.5%");
        assert_eq!(Column::BuildTime.format(&sum, &EN), "6.1m");
        assert_eq!(Column::DeploysPerWeek.format(&sum, &ZH_TW), "2.5/週");
        assert_eq!(Column::LeadTime.format(&sum, &EN), "—");
        assert_eq!(format_seconds(45.0), "45s");
        assert_eq!(format_seconds(7200.0), "2.0h");
    }
}
