package fetching

import (
	"testing"
	"time"
)

func TestNewMonthRange_HappyMonth(t *testing.T) {
	m := NewMonthRange(2026, time.May)
	if m.Start.Format(time.RFC3339) != "2026-05-01T00:00:00Z" {
		t.Fatalf("start %v", m.Start)
	}
	if m.End.Format(time.RFC3339) != "2026-06-01T00:00:00Z" {
		t.Fatalf("end %v", m.End)
	}
}

func TestNewMonthRange_DecemberRollsOverYear(t *testing.T) {
	m := NewMonthRange(2026, time.December)
	if m.End.Format(time.RFC3339) != "2027-01-01T00:00:00Z" {
		t.Fatalf("expected jan next year, got %v", m.End)
	}
}

func TestNewMonthRange_FebruaryLeapYear(t *testing.T) {
	m := NewMonthRange(2024, time.February)
	if m.End.Format(time.RFC3339) != "2024-03-01T00:00:00Z" {
		t.Fatalf("expected mar, got %v", m.End)
	}
}

func TestMonthRange_Contains(t *testing.T) {
	m := NewMonthRange(2026, time.May)

	mid := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	if !m.Contains(mid) {
		t.Fatalf("mid-month should be inside")
	}
	if !m.Contains(m.Start) {
		t.Fatalf("start is inclusive")
	}
	if m.Contains(m.End) {
		t.Fatalf("end is exclusive")
	}

	before := time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)
	if m.Contains(before) {
		t.Fatalf("before should not be inside")
	}
}

func TestParseMonthRange(t *testing.T) {
	m, err := ParseMonthRange("2026-05")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if m.String() != "2026-05" {
		t.Fatalf("round-trip got %q", m.String())
	}

	if _, err := ParseMonthRange("nope"); err == nil {
		t.Fatalf("expected parse error")
	}
}

