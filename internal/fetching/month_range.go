package fetching

import (
	"fmt"
	"time"
)

// MonthRange is the half-open UTC interval [Start, End) covering a single
// calendar month. The orchestrator uses it as the partition unit for both
// CI builds and PRs.
type MonthRange struct {
	Start time.Time
	End   time.Time
}

// NewMonthRange returns the [first-of-month, first-of-next-month) range in UTC.
func NewMonthRange(year int, month time.Month) MonthRange {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return MonthRange{Start: start, End: end}
}

// ParseMonthRange accepts "YYYY-MM".
func ParseMonthRange(s string) (MonthRange, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return MonthRange{}, fmt.Errorf("month range: %w", err)
	}
	return NewMonthRange(t.Year(), t.Month()), nil
}

func (m MonthRange) Contains(t time.Time) bool {
	tu := t.UTC()
	return !tu.Before(m.Start) && tu.Before(m.End)
}

func (m MonthRange) String() string {
	return m.Start.Format("2006-01")
}

