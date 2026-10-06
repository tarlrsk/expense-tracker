package domain

import (
	"fmt"
	"time"
)

// Month is a calendar month, such as 2026-10. The zero Month is no month; only NewMonth and
// AddMonths make one. Months compare with ==.
type Month struct {
	year  int
	month time.Month
}

// NewMonth is the month of year y. Like time.Date it normalises a month outside January to
// December: NewMonth(2026, 13) is 2027-01 and NewMonth(2026, 0) is 2025-12. Checking a client's
// input is the caller's job.
func NewMonth(y int, m time.Month) Month {
	t := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	return Month{year: t.Year(), month: t.Month()}
}

// IsZero reports whether m is the zero Month.
func (m Month) IsZero() bool { return m.month == 0 }

// Year is the month's year.
func (m Month) Year() int { return m.year }

// Month is the month of the year.
func (m Month) Month() time.Month { return m.month }

// AddMonths is the month n months after m (before it when n is negative), across years:
// 2026-01 with -1 is 2025-12.
func (m Month) AddMonths(n int) Month {
	return NewMonth(m.year, m.month+time.Month(n))
}

// String writes the month as YYYY-MM; "" for the zero Month.
func (m Month) String() string {
	if m.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d", m.year, int(m.month))
}

// FirstDay is the month's first day as YYYY-MM-DD, the start of the month's date range in SQL
// (included).
func (m Month) FirstDay() string { return m.String() + "-01" }

// NextFirstDay is the first day of the following month as YYYY-MM-DD, the end of the month's
// date range in SQL (excluded).
func (m Month) NextFirstDay() string { return m.AddMonths(1).FirstDay() }
