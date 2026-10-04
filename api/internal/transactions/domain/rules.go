package domain

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// Amount is an amount of money in hundredths of the currency unit (satang for THB). It is
// carried exactly: never as a float (ADR-0071). Only ParseAmount makes one from text.
type Amount int64

// MaxAmount is 9,999,999,999.99, the largest value of the numeric(12,2) column.
const MaxAmount Amount = 999_999_999_999

// maxAmountUnits is how many digits the whole-unit part of MaxAmount has.
const maxAmountUnits = 10

// amountPattern is digits with an optional fraction of one or two digits: no sign, exponent,
// spaces or leading or trailing point.
var amountPattern = regexp.MustCompile(`^([0-9]+)(?:\.([0-9]{1,2}))?$`)

// ParseAmount reads an amount written as text, such as "145", "145.5" or "145.50". It must be
// more than 0 and at most MaxAmount; more than two decimals is refused, never rounded. Otherwise
// it returns an invalid_input error that does not quote s.
func ParseAmount(s string) (Amount, error) {
	m := amountPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, apperr.New(apperr.InvalidInput, AmountRuleMessage)
	}
	units := strings.TrimLeft(m[1], "0")
	if len(units) > maxAmountUnits {
		return 0, apperr.New(apperr.InvalidInput, AmountRuleMessage)
	}
	var whole int64
	if units != "" {
		n, err := strconv.ParseInt(units, 10, 64)
		if err != nil {
			return 0, apperr.New(apperr.InvalidInput, AmountRuleMessage)
		}
		whole = n
	}
	frac := m[2]
	for len(frac) < 2 {
		frac += "0"
	}
	cents, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, apperr.New(apperr.InvalidInput, AmountRuleMessage)
	}
	a := Amount(whole*100 + cents)
	if a <= 0 || a > MaxAmount {
		return 0, apperr.New(apperr.InvalidInput, AmountRuleMessage)
	}
	return a, nil
}

// String writes the amount with exactly two decimals, such as "145.00"; Postgres reads it as a
// numeric without loss.
func (a Amount) String() string {
	cents := int64(a) % 100
	s := strconv.FormatInt(int64(a)/100, 10) + "."
	if cents < 10 {
		s += "0"
	}
	return s + strconv.FormatInt(cents, 10)
}

// Date is a calendar day. Only ParseDate, DateOf, AddDays and the period helpers make one; the zero
// Date is no day.
type Date struct {
	// t is midnight UTC of the day.
	t time.Time
}

const dateLayout = "2006-01-02"

// MinDate is the earliest occurred_on (ADR-0071).
var MinDate = Date{t: time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)}

// ParseDate reads a real calendar date written YYYY-MM-DD (four-digit year, two-digit month and
// day). ok is false otherwise, for 2026-02-30 too.
func ParseDate(s string) (d Date, ok bool) {
	if len(s) != len(dateLayout) {
		return Date{}, false
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.Format(dateLayout) != s {
		return Date{}, false
	}
	return Date{t: t}, true
}

// DateOf is the calendar day of t in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{t: time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

// Today is the day now falls on in the app time zone (ADR-0042).
func Today(now time.Time, zone *time.Location) Date {
	return DateOf(now.In(zone))
}

// String writes the date as YYYY-MM-DD; "" for the zero Date.
func (d Date) String() string {
	if d.t.IsZero() {
		return ""
	}
	return d.t.Format(dateLayout)
}

// IsZero reports whether d is the zero Date.
func (d Date) IsZero() bool { return d.t.IsZero() }

// Compare returns -1, 0 or +1 as d is before, equal to or after o.
func (d Date) Compare(o Date) int { return d.t.Compare(o.t) }

// AddDays returns the day n days after d (before it when n is negative). The zero Date stays the
// zero Date.
func (d Date) AddDays(n int) Date {
	if d.t.IsZero() {
		return Date{}
	}
	return Date{t: d.t.AddDate(0, 0, n)}
}

// MaxDate is the latest occurred_on on day today: one year later (a 29 February gives 1 March).
func MaxDate(today Date) Date { return Date{t: today.t.AddDate(1, 0, 0)} }

// ParseOccurredOn reads occurred_on and checks it lies from MinDate to MaxDate(today), both
// included. Otherwise it returns an invalid_input error that does not quote s.
func ParseOccurredOn(s string, today Date) (Date, error) {
	d, ok := ParseDate(s)
	if !ok || d.Compare(MinDate) < 0 || d.Compare(MaxDate(today)) > 0 {
		return Date{}, apperr.New(apperr.InvalidInput, DateRuleMessage)
	}
	return d, nil
}

// CurrencyTHB is the only currency for now (ADR-0041).
const CurrencyTHB = "THB"

// ParseCurrency returns currency when it is an accepted one (THB only), or an invalid_input
// error.
func ParseCurrency(currency string) (string, error) {
	if currency != CurrencyTHB {
		return "", apperr.New(apperr.InvalidInput, CurrencyMessage)
	}
	return currency, nil
}

// Source says how a transaction was entered. It never changes after creation.
type Source string

// The sources (migration 0004). The API creates only manual ones for now; text, scan and csv
// come with smart entry, scanning and import.
const (
	SourceManual Source = "manual"
	SourceText   Source = "text"
	SourceScan   Source = "scan"
	SourceCSV    Source = "csv"
)

// ParseCreateSource returns the source a create may set: only manual for now (ADR-0071).
func ParseCreateSource(source string) (Source, error) {
	if Source(source) != SourceManual {
		return "", apperr.New(apperr.InvalidInput, SourceMessage)
	}
	return SourceManual, nil
}

// ParseSourceFilter returns source when it is one of the sources, for the list filter.
func ParseSourceFilter(source string) (Source, error) {
	switch s := Source(source); s {
	case SourceManual, SourceText, SourceScan, SourceCSV:
		return s, nil
	default:
		return "", apperr.New(apperr.InvalidInput, FilterSourceMessage)
	}
}

// Text limits (ADR-0071), in characters (runes).
const (
	MaxMerchantLength = 100
	MaxNoteLength     = 500
)

// NormalizeMerchant trims whitespace around merchant; the result has at most MaxMerchantLength
// characters and no control character. It may be empty.
func NormalizeMerchant(merchant string) (string, error) {
	merchant = strings.TrimSpace(merchant)
	if utf8.RuneCountInString(merchant) > MaxMerchantLength || strings.ContainsFunc(merchant, unicode.IsControl) {
		return "", apperr.New(apperr.InvalidInput, MerchantRuleMessage)
	}
	return merchant, nil
}

// NormalizeNote trims whitespace around note; the result has at most MaxNoteLength characters
// and no control character other than a newline. Windows line endings (\r\n) become \n. It may be
// empty.
func NormalizeNote(note string) (string, error) {
	note = strings.TrimSpace(strings.ReplaceAll(note, "\r\n", "\n"))
	badControl := func(r rune) bool { return r != '\n' && unicode.IsControl(r) }
	if utf8.RuneCountInString(note) > MaxNoteLength || strings.ContainsFunc(note, badControl) {
		return "", apperr.New(apperr.InvalidInput, NoteRuleMessage)
	}
	return note, nil
}

// canonicalUUIDLength is the length of the hyphenated form 01890a5d-ac96-774b-bcce-b302099a8057.
const canonicalUUIDLength = 36

// parseCanonicalUUID reads a UUID in its hyphenated 36-character form only; uuid.Parse also takes
// braces, a urn:uuid: prefix and the form without hyphens.
func parseCanonicalUUID(s string) (uuid.UUID, bool) {
	if len(s) != canonicalUUIDLength {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	return id, err == nil
}

// ParseNewID reads the client-made id of a new transaction: a UUID version 7 in hyphenated form
// (ADR-0040, ADR-0071). Otherwise it returns an invalid_input error with IDRuleMessage.
func ParseNewID(s string) (uuid.UUID, error) {
	id, ok := parseCanonicalUUID(s)
	if !ok || id.Version() != 7 || id.Variant() != uuid.RFC4122 {
		return uuid.Nil, apperr.New(apperr.InvalidInput, IDRuleMessage)
	}
	return id, nil
}

// ParseCategoryID reads the category_id of a create or update. A malformed id is answered like
// an unknown category (CategoryMessage).
func ParseCategoryID(s string) (uuid.UUID, error) {
	id, ok := parseCanonicalUUID(s)
	if !ok {
		return uuid.Nil, apperr.New(apperr.InvalidInput, CategoryMessage)
	}
	return id, nil
}

// ParseCategoryFilter reads the category filter of the list.
func ParseCategoryFilter(s string) (uuid.UUID, error) {
	id, ok := parseCanonicalUUID(s)
	if !ok {
		return uuid.Nil, apperr.New(apperr.InvalidInput, FilterCategoryMessage)
	}
	return id, nil
}
