package domain

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// Page sizes of the list (ADR-0071).
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// Period is the date range of a list, both ends included; a nil end is open.
type Period struct {
	From *Date
	To   *Date
}

const monthLayout = "2006-01"

// ParsePeriod reads the list's period from the month, from and to parameters; a nil parameter
// was not sent (ADR-0071):
//   - month (YYYY-MM) is its first to its last day, and cannot be combined with from or to;
//   - from and to (YYYY-MM-DD) are included, and either may be sent alone; from after to is
//     refused;
//   - none of them is every date.
func ParsePeriod(month, from, to *string) (Period, error) {
	bad := apperr.New(apperr.InvalidInput, PeriodMessage)
	if month != nil {
		if from != nil || to != nil || len(*month) != len(monthLayout) {
			return Period{}, bad
		}
		t, err := time.Parse(monthLayout, *month)
		if err != nil || t.Format(monthLayout) != *month {
			return Period{}, bad
		}
		first, last := Date{t: t}, Date{t: t.AddDate(0, 1, -1)}
		return Period{From: &first, To: &last}, nil
	}
	var p Period
	for _, end := range []struct {
		raw *string
		dst **Date
	}{{from, &p.From}, {to, &p.To}} {
		if end.raw == nil {
			continue
		}
		d, ok := ParseDate(*end.raw)
		if !ok {
			return Period{}, bad
		}
		*end.dst = &d
	}
	if p.From != nil && p.To != nil && p.From.Compare(*p.To) > 0 {
		return Period{}, bad
	}
	return p, nil
}

// ParseLimit reads the page size: a whole number from 1 to MaxPageSize written in digits only.
// A nil limit is DefaultPageSize.
func ParseLimit(limit *string) (int, error) {
	if limit == nil {
		return DefaultPageSize, nil
	}
	s := *limit
	if s == "" || len(s) > 3 || strings.Trim(s, "0123456789") != "" {
		return 0, apperr.New(apperr.InvalidInput, LimitMessage)
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > MaxPageSize {
		return 0, apperr.New(apperr.InvalidInput, LimitMessage)
	}
	return n, nil
}

// Cursor is a position in a list: the list continues after the transaction of this date and id
// in the order occurred_on descending, then id descending (keyset paging, ADR-0071). It carries
// no secret and gives no access: it only positions inside the caller's own rows.
type Cursor struct {
	OccurredOn Date
	ID         uuid.UUID
}

// cursorSeparator joins the date and the id inside a cursor.
const cursorSeparator = "_"

// CursorAfter is the cursor that continues the list after t.
func CursorAfter(t Transaction) Cursor {
	return Cursor{OccurredOn: t.OccurredOn, ID: t.ID}
}

// String encodes the cursor as an opaque, URL-safe text (base64url without padding of
// "YYYY-MM-DD_<id>"). Clients must not build or read it.
func (c Cursor) String() string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.OccurredOn.String() + cursorSeparator + c.ID.String()))
}

// ParseCursor reads a cursor made by Cursor.String, or returns an invalid_input error.
func ParseCursor(s string) (Cursor, error) {
	bad := apperr.New(apperr.InvalidInput, CursorMessage)
	raw, err := base64.RawURLEncoding.Strict().DecodeString(s)
	if err != nil {
		return Cursor{}, bad
	}
	date, id, ok := strings.Cut(string(raw), cursorSeparator)
	if !ok {
		return Cursor{}, bad
	}
	d, ok := ParseDate(date)
	if !ok {
		return Cursor{}, bad
	}
	u, ok := parseCanonicalUUID(id)
	if !ok {
		return Cursor{}, bad
	}
	return Cursor{OccurredOn: d, ID: u}, nil
}
