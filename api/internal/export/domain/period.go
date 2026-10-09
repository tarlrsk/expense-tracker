package domain

import (
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	txdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// PeriodMessage is the client message for a bad from or to; it never quotes the value.
const PeriodMessage = "from and to must be dates written YYYY-MM-DD, from not after to"

// Period is the date range of an export, both ends included; a nil end is open, so the zero
// Period is every date.
type Period struct {
	From *txdomain.Date
	To   *txdomain.Date
}

// ParsePeriod reads the from and to parameters; a nil parameter was not sent. Each is a real
// date written YYYY-MM-DD, either may be sent alone, and from after to is refused (the rules of
// the transaction list, ADR-0071, without its month parameter).
func ParsePeriod(from, to *string) (Period, error) {
	var p Period
	for _, end := range []struct {
		raw *string
		dst **txdomain.Date
	}{{from, &p.From}, {to, &p.To}} {
		if end.raw == nil {
			continue
		}
		d, ok := txdomain.ParseDate(*end.raw)
		if !ok {
			return Period{}, apperr.New(apperr.InvalidInput, PeriodMessage)
		}
		*end.dst = &d
	}
	if p.From != nil && p.To != nil && p.From.Compare(*p.To) > 0 {
		return Period{}, apperr.New(apperr.InvalidInput, PeriodMessage)
	}
	return p, nil
}

// FileName is the name the file is downloaded under: satang-transactions.csv for every date,
// otherwise with the period's ends, such as
// satang-transactions-2026-10-01-to-2026-10-31.csv, satang-transactions-from-2026-10-01.csv or
// satang-transactions-to-2026-10-31.csv. It holds only ASCII letters, digits and dashes.
func FileName(p Period) string {
	const base, ext = "satang-transactions", ".csv"
	switch {
	case p.From != nil && p.To != nil:
		return base + "-" + p.From.String() + "-to-" + p.To.String() + ext
	case p.From != nil:
		return base + "-from-" + p.From.String() + ext
	case p.To != nil:
		return base + "-to-" + p.To.String() + ext
	default:
		return base + ext
	}
}
