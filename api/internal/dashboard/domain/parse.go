package domain

import (
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// MonthMessage is the client message for a month parameter that is malformed or out of range. It
// never quotes the value sent.
const MonthMessage = "month must be written YYYY-MM, from 2000-01 to one year after this month"

// MinMonth is the earliest month the dashboard accepts: the month of the earliest occurred_on
// (ADR-0071).
var MinMonth = NewMonth(2000, time.January)

// MaxMonth is the latest month the dashboard accepts when current is this month: one year later,
// like the latest occurred_on (ADR-0071).
func MaxMonth(current Month) Month { return current.AddMonths(12) }

// ParseMonth reads a real month written exactly YYYY-MM (four ASCII digits, a hyphen, two ASCII
// digits from 01 to 12, nothing before or after) that lies from MinMonth to MaxMonth(current),
// both included. Otherwise it returns an invalid_input error with MonthMessage.
func ParseMonth(s string, current Month) (Month, error) {
	bad := apperr.New(apperr.InvalidInput, MonthMessage)
	if len(s) != len("2006-01") || s[4] != '-' {
		return Month{}, bad
	}
	year, ok := digits(s[:4])
	if !ok {
		return Month{}, bad
	}
	month, ok := digits(s[5:])
	if !ok || month < 1 || month > 12 {
		return Month{}, bad
	}
	m := NewMonth(year, time.Month(month))
	if m.index() < MinMonth.index() || m.index() > MaxMonth(current).index() {
		return Month{}, bad
	}
	return m, nil
}

// CurrentMonth is the month now falls in in the app time zone (ADR-0042).
func CurrentMonth(now time.Time, zone *time.Location) Month {
	t := now.In(zone)
	return NewMonth(t.Year(), t.Month())
}

// digits reads s as a whole number written in ASCII digits only; ok is false for anything else.
func digits(s string) (n int, ok bool) {
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, s != ""
}

// index counts months from year 0, so months compare as whole numbers.
func (m Month) index() int { return m.year*12 + int(m.month) - 1 }
