package domain

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewMonth(t *testing.T) {
	tests := []struct {
		name  string
		year  int
		month time.Month
		want  string
	}{
		{"a plain month", 2026, time.October, "2026-10"},
		{"January", 2026, time.January, "2026-01"},
		{"December", 2025, time.December, "2025-12"},
		{"month 13 is January of the next year", 2026, 13, "2027-01"},
		{"month 0 is December of the year before", 2026, 0, "2025-12"},
		{"a year below 1000 is padded", 999, time.March, "0999-03"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMonth(tt.year, tt.month)
			if got := m.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if m.IsZero() {
				t.Error("IsZero() = true")
			}
		})
	}
}

func TestMonthAccessors(t *testing.T) {
	m := NewMonth(2024, time.February)
	if m.Year() != 2024 || m.Month() != time.February {
		t.Errorf("Year, Month = %d, %v", m.Year(), m.Month())
	}
	if m != NewMonth(2024, time.February) {
		t.Error("equal months do not compare equal")
	}
	if m == NewMonth(2025, time.February) {
		t.Error("different years compare equal")
	}
}

func TestZeroMonth(t *testing.T) {
	var m Month
	if !m.IsZero() || m.String() != "" {
		t.Errorf("zero Month: IsZero %v, String %q", m.IsZero(), m.String())
	}
}

func TestAddMonths(t *testing.T) {
	tests := []struct {
		name string
		from Month
		n    int
		want string
	}{
		{"zero", NewMonth(2026, time.March), 0, "2026-03"},
		{"one back inside the year", NewMonth(2026, time.March), -1, "2026-02"},
		{"one back across the year", NewMonth(2026, time.January), -1, "2025-12"},
		{"five back across the year", NewMonth(2026, time.March), -5, "2025-10"},
		{"five back from January", NewMonth(2026, time.January), -5, "2025-08"},
		{"twelve back", NewMonth(2026, time.March), -12, "2025-03"},
		{"twenty-five back", NewMonth(2026, time.March), -25, "2024-02"},
		{"one on across the year", NewMonth(2025, time.December), 1, "2026-01"},
		{"fourteen on", NewMonth(2025, time.December), 14, "2027-02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.from.AddMonths(tt.n).String(); got != tt.want {
				t.Errorf("%s.AddMonths(%d) = %s, want %s", tt.from, tt.n, got, tt.want)
			}
		})
	}
}

func TestMonthDays(t *testing.T) {
	tests := []struct {
		month     Month
		first     string
		nextFirst string
	}{
		{NewMonth(2026, time.April), "2026-04-01", "2026-05-01"},
		{NewMonth(2024, time.February), "2024-02-01", "2024-03-01"},
		{NewMonth(2025, time.December), "2025-12-01", "2026-01-01"},
		{NewMonth(2026, time.January), "2026-01-01", "2026-02-01"},
	}
	for _, tt := range tests {
		t.Run(tt.month.String(), func(t *testing.T) {
			if got := tt.month.FirstDay(); got != tt.first {
				t.Errorf("FirstDay() = %s, want %s", got, tt.first)
			}
			if got := tt.month.NextFirstDay(); got != tt.nextFirst {
				t.Errorf("NextFirstDay() = %s, want %s", got, tt.nextFirst)
			}
		})
	}
}

func TestMoneyString(t *testing.T) {
	tests := []struct {
		m    Money
		want string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{10, "0.10"},
		{30, "0.30"},
		{100, "1.00"},
		{145000, "1450.00"},
		{14550, "145.50"},
		{999_999_999_999, "9999999999.99"},
		{-1, "-0.01"},
		{-5, "-0.05"},
		{-100, "-1.00"},
		{-1250, "-12.50"},
		{math.MaxInt64, "92233720368547758.07"},
		{math.MinInt64, "-92233720368547758.08"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.m.String(); got != tt.want {
				t.Errorf("Money(%d).String() = %q, want %q", int64(tt.m), got, tt.want)
			}
		})
	}
}

func TestDifference(t *testing.T) {
	tests := []struct {
		name   string
		totals MonthTotals
		want   Money
	}{
		{"nothing", MonthTotals{}, 0},
		{"more received", MonthTotals{Spent: 14550, Income: 5_000_000}, 4_985_450},
		{"more spent", MonthTotals{Spent: 2000, Income: 750}, -1250},
		{"only spent", MonthTotals{Spent: 30}, -30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.totals.Difference(); got != tt.want {
				t.Errorf("Difference() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestOwnerScope(t *testing.T) {
	id := uuid.New()
	if got := OwnerScope(id); got != (Scope{OwnerID: id}) {
		t.Errorf("OwnerScope = %+v", got)
	}
}
