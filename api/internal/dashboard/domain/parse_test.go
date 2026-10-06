package domain

import (
	"errors"
	"testing"
	"time"
	_ "time/tzdata" // the zone database, as in the API binary

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

func TestParseMonth(t *testing.T) {
	current := NewMonth(2026, time.October)
	tests := []struct {
		name string
		in   string
		want string // "" when refused
	}{
		{"a plain month", "2026-10", "2026-10"},
		{"January", "2026-01", "2026-01"},
		{"December", "2025-12", "2025-12"},
		{"the earliest month", "2000-01", "2000-01"},
		{"one year after this month", "2027-10", "2027-10"},
		{"before the earliest month", "1999-12", ""},
		{"thirteen months after this month", "2027-11", ""},
		{"a year far ahead", "9999-12", ""},
		{"year zero", "0000-01", ""},
		{"month 13", "2026-13", ""},
		{"month 00", "2026-00", ""},
		{"a one-digit month", "2026-1", ""},
		{"a two-digit year", "26-10", ""},
		{"a date", "2026-10-01", ""},
		{"a slash", "2026/10", ""},
		{"empty", "", ""},
		{"a leading space", " 2026-10", ""},
		{"a trailing space", "2026-10 ", ""},
		{"Thai digits", "๒๐๒๖-๑๐", ""},
		{"full-width digits", "２０２６-１０", ""},
		{"a sign", "+026-10", ""},
		{"a negative month", "2026--1", ""},
		{"letters", "abcd-ef", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := ParseMonth(tt.in, current)
			if tt.want == "" {
				if apperr.KindOf(err) != apperr.InvalidInput {
					t.Fatalf("ParseMonth(%q) = %v, %v; want invalid_input", tt.in, m, err)
				}
				var e *apperr.Error
				if !errors.As(err, &e) || e.Message != MonthMessage {
					t.Errorf("message = %v, want %q", err, MonthMessage)
				}
				if !m.IsZero() {
					t.Errorf("month = %v, want the zero Month", m)
				}
				return
			}
			if err != nil || m.String() != tt.want {
				t.Errorf("ParseMonth(%q) = %v, %v; want %s", tt.in, m, err, tt.want)
			}
		})
	}
}

func TestParseMonthRangeFollowsCurrent(t *testing.T) {
	tests := []struct {
		name    string
		current Month
		in      string
		ok      bool
	}{
		{"one year after December", NewMonth(2026, time.December), "2027-12", true},
		{"thirteen months after December", NewMonth(2026, time.December), "2028-01", false},
		{"one year after January", NewMonth(2026, time.January), "2027-01", true},
		{"thirteen months after January", NewMonth(2026, time.January), "2027-02", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMonth(tt.in, tt.current)
			if (err == nil) != tt.ok {
				t.Errorf("ParseMonth(%q, %s) error = %v, want ok %v", tt.in, tt.current, err, tt.ok)
			}
		})
	}
}

func TestCurrentMonth(t *testing.T) {
	bangkok, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		t.Fatalf("load Asia/Bangkok: %v", err)
	}
	tests := []struct {
		name string
		now  time.Time
		zone *time.Location
		want string
	}{
		{"evening of the last day in UTC is the next month in Bangkok", time.Date(2026, time.October, 31, 18, 0, 0, 0, time.UTC), bangkok, "2026-11"},
		{"the same instant in UTC", time.Date(2026, time.October, 31, 18, 0, 0, 0, time.UTC), time.UTC, "2026-10"},
		{"just before midnight in Bangkok", time.Date(2026, time.October, 31, 16, 59, 59, 0, time.UTC), bangkok, "2026-10"},
		{"new year in Bangkok", time.Date(2026, time.December, 31, 17, 0, 0, 0, time.UTC), bangkok, "2027-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CurrentMonth(tt.now, tt.zone).String(); got != tt.want {
				t.Errorf("CurrentMonth = %s, want %s", got, tt.want)
			}
		})
	}
}
