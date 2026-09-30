//! Calendar month arithmetic for the `YYYY-MM` windows the API takes.

use std::fmt;
use std::str::FromStr;
use std::time::{SystemTime, UNIX_EPOCH};

/// A calendar month, e.g. 2026-05.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct Month {
    year: i32,
    month: u32, // 1..=12
}

impl Month {
    pub fn new(year: i32, month: u32) -> Option<Self> {
        (1..=12).contains(&month).then_some(Self { year, month })
    }

    /// The current month in UTC, matching the server's default window.
    pub fn current() -> Self {
        let secs = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0);
        Self::from_unix_days((secs / 86_400) as i64)
    }

    /// Converts days since 1970-01-01 to its month, using Howard
    /// Hinnant's civil-from-days algorithm.
    fn from_unix_days(days: i64) -> Self {
        let z = days + 719_468;
        let era = z.div_euclid(146_097);
        let doe = z.rem_euclid(146_097);
        let yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365;
        let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
        let mp = (5 * doy + 2) / 153;
        let month = if mp < 10 { mp + 3 } else { mp - 9 } as u32;
        let year = (yoe + era * 400 + i64::from(month <= 2)) as i32;
        Self { year, month }
    }

    /// Shifts by `n` months (negative goes back).
    /// January of this month's year.
    pub fn january(self) -> Self {
        Self {
            year: self.year,
            month: 1,
        }
    }

    pub fn add(self, n: i32) -> Self {
        let index = self.year * 12 + self.month as i32 - 1 + n;
        Self {
            year: index.div_euclid(12),
            month: index.rem_euclid(12) as u32 + 1,
        }
    }

    pub fn next(self) -> Self {
        self.add(1)
    }

    pub fn prev(self) -> Self {
        self.add(-1)
    }
}

impl fmt::Display for Month {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:04}-{:02}", self.year, self.month)
    }
}

impl FromStr for Month {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let bad = || format!("expected YYYY-MM, got {s:?}");
        let (y, m) = s.trim().split_once('-').ok_or_else(bad)?;
        if y.len() != 4 || m.len() != 2 {
            return Err(bad());
        }
        let year = y.parse().map_err(|_| bad())?;
        let month = m.parse().map_err(|_| bad())?;
        Self::new(year, month).ok_or_else(bad)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn m(s: &str) -> Month {
        s.parse().unwrap()
    }

    #[test]
    fn parses_and_formats() {
        assert_eq!(m("2026-05").to_string(), "2026-05");
        assert_eq!(m(" 0999-12 ").to_string(), "0999-12");
        for bad in [
            "2026-5", "2026-13", "2026-00", "26-05", "2026/05", "", "abcd-ef",
        ] {
            assert!(bad.parse::<Month>().is_err(), "{bad} should not parse");
        }
    }

    #[test]
    fn adds_across_year_boundaries() {
        assert_eq!(m("2026-12").next(), m("2027-01"));
        assert_eq!(m("2026-01").prev(), m("2025-12"));
        assert_eq!(m("2026-05").add(-17), m("2024-12"));
        assert_eq!(m("2026-05").add(24), m("2028-05"));
    }

    #[test]
    fn converts_unix_days() {
        assert_eq!(Month::from_unix_days(0), m("1970-01"));
        // 2026-05-17 is day 20590 since the epoch.
        assert_eq!(Month::from_unix_days(20_590), m("2026-05"));
        // 2024-02-29 (leap day) is day 19782.
        assert_eq!(Month::from_unix_days(19_782), m("2024-02"));
        assert_eq!(Month::from_unix_days(19_783), m("2024-03"));
    }
}
