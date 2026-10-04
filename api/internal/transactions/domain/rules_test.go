package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the zone database, as in the API binary

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in   string
		want Amount // 0: refused
		text string
	}{
		{"145", 14500, "145.00"}, {"145.5", 14550, "145.50"}, {"145.05", 14505, "145.05"}, {"0.01", 1, "0.01"},
		{"0.1", 10, "0.10"}, {"9999999999.99", MaxAmount, "9999999999.99"}, {"0009999999999.99", MaxAmount, "9999999999.99"},
		{"1", 100, "1.00"}, {"10.00", 1000, "10.00"},
		{"0", 0, ""}, {"0.00", 0, ""}, {"00", 0, ""}, {"-1", 0, ""}, {"+1", 0, ""}, {"1.234", 0, ""}, {"1.000", 0, ""},
		{"1e3", 0, ""}, {"1E3", 0, ""}, {" 1", 0, ""}, {"1 ", 0, ""}, {"1.", 0, ""}, {".5", 0, ""}, {"", 0, ""},
		{".", 0, ""}, {"1,5", 0, ""}, {"1_000", 0, ""}, {"10000000000.00", 0, ""}, {"10000000000", 0, ""},
		{"99999999999999999999999", 0, ""}, {"１", 0, ""}, {"๑", 0, ""}, {"0x1", 0, ""}, {"1.5\n", 0, ""},
	}
	for _, tt := range tests {
		got, err := ParseAmount(tt.in)
		if tt.want == 0 {
			if apperr.KindOf(err) != apperr.InvalidInput {
				t.Errorf("ParseAmount(%q) = %v, %v; want invalid_input", tt.in, got, err)
			} else if err.Error() != "invalid_input: "+AmountRuleMessage {
				t.Errorf("ParseAmount(%q): the error is not the fixed rule message: %v", tt.in, err)
			}
			continue
		}
		if err != nil || got != tt.want || got.String() != tt.text {
			t.Errorf("ParseAmount(%q) = %d (%s), %v; want %d (%s)", tt.in, got, got, err, tt.want, tt.text)
		}
	}
}

func TestDates(t *testing.T) {
	bangkok, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		t.Fatal(err)
	}
	// 17:30 UTC on 2 October is 00:30 on 3 October in Bangkok.
	now := time.Date(2026, time.October, 2, 17, 30, 0, 0, time.UTC)
	if got := Today(now, bangkok).String(); got != "2026-10-03" {
		t.Errorf("Today in Bangkok = %s", got)
	}
	if got := Today(now, time.UTC).String(); got != "2026-10-02" {
		t.Errorf("Today in UTC = %s", got)
	}
	leap, _ := ParseDate("2028-02-29")
	if got := MaxDate(leap).String(); got != "2029-03-01" {
		t.Errorf("MaxDate(2028-02-29) = %s", got)
	}

	today := Today(now, bangkok)
	for _, c := range []struct {
		in string
		ok bool
	}{
		{"2000-01-01", true}, {"1999-12-31", false}, {"2027-10-03", true}, {"2027-10-04", false},
		{"2024-02-29", true}, {"2026-02-29", false}, {"2026-02-30", false}, {"2026-2-3", false}, {"2026-02-03 ", false},
		{"2026-02-03T00:00:00Z", false}, {"", false}, {"02/03/2026", false}, {"+2026-02-03", false},
	} {
		d, err := ParseOccurredOn(c.in, today)
		if c.ok != (err == nil) || (c.ok && d.String() != c.in) {
			t.Errorf("ParseOccurredOn(%q) = %s, %v; want ok %v", c.in, d, err, c.ok)
		}
		if err != nil && (apperr.KindOf(err) != apperr.InvalidInput || (c.in != "" && strings.Contains(err.Error(), c.in))) {
			t.Errorf("ParseOccurredOn(%q): error %v", c.in, err)
		}
	}
	if _, err := ParseOccurredOn("2027-10-03", Today(now, time.UTC)); err == nil {
		t.Error("in UTC, 2027-10-03 is more than a year after today and must be refused")
	}
}

func TestAddDays(t *testing.T) {
	for _, c := range []struct {
		from string
		n    int
		want string
	}{
		{"2026-10-04", 0, "2026-10-04"}, {"2026-10-04", -1, "2026-10-03"}, {"2026-10-04", -2, "2026-10-02"},
		{"2026-10-01", -1, "2026-09-30"}, {"2026-01-01", -1, "2025-12-31"}, {"2028-03-01", -1, "2028-02-29"},
		{"2026-03-01", -1, "2026-02-28"}, {"2026-12-31", 1, "2027-01-01"},
	} {
		d, ok := ParseDate(c.from)
		if !ok {
			t.Fatalf("ParseDate(%q)", c.from)
		}
		if got := d.AddDays(c.n).String(); got != c.want {
			t.Errorf("%s.AddDays(%d) = %s, want %s", c.from, c.n, got, c.want)
		}
	}
	if got := (Date{}).AddDays(-1); !got.IsZero() {
		t.Errorf("zero Date.AddDays(-1) = %s, want the zero Date", got)
	}
}

func TestParseNewID(t *testing.T) {
	v7 := uuid.Must(uuid.NewV7())
	for _, c := range []struct {
		in string
		ok bool
	}{
		{v7.String(), true}, {strings.ToUpper(v7.String()), true},
		{uuid.NewString(), false}, {uuid.Nil.String(), false}, {"", false}, {"x", false},
		{strings.ReplaceAll(v7.String(), "-", ""), false}, {"{" + v7.String() + "}", false}, {"urn:uuid:" + v7.String(), false},
		// version 7 with the wrong variant bits
		{v7.String()[:19] + "0" + v7.String()[20:], false},
	} {
		id, err := ParseNewID(c.in)
		if c.ok != (err == nil) || (c.ok && id != v7) {
			t.Errorf("ParseNewID(%q) = %s, %v; want ok %v", c.in, id, err, c.ok)
		}
	}
}

func TestText(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"  7-Eleven ", "7-Eleven", true}, {"", "", true}, {strings.Repeat("ก", 100), strings.Repeat("ก", 100), true},
		{strings.Repeat("ก", 101), "", false}, {"a\nb", "", false}, {"a\tb", "", false}, {"a\u0085b", "", false},
	} {
		got, err := NormalizeMerchant(c.in)
		if c.ok != (err == nil) || got != c.want {
			t.Errorf("NormalizeMerchant(%q) = %q, %v", c.in, got, err)
		}
	}
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{" a\nb \n", "a\nb", true}, {strings.Repeat("x", 500), strings.Repeat("x", 500), true},
		{strings.Repeat("x", 501), "", false}, {"a\r\nb", "a\nb", true}, {"a\rb", "", false}, {"a\tb", "", false}, {"a\x00", "", false},
	} {
		got, err := NormalizeNote(c.in)
		if c.ok != (err == nil) || got != c.want {
			t.Errorf("NormalizeNote(%q) = %q, %v", c.in, got, err)
		}
	}
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{" grab 145 ", "grab 145", true}, {"", "", true}, {"coffee 60\ngrab 145", "coffee 60\ngrab 145", true},
		{strings.Repeat("ก", 1000), strings.Repeat("ก", 1000), true}, {" " + strings.Repeat("ก", 1000) + "\n", strings.Repeat("ก", 1000), true},
		{strings.Repeat("ก", 1001), "", false}, {"a\r\nb", "a\nb", true}, {"a\rb", "", false}, {"a\tb", "", false}, {"a\x00", "", false},
		{"a\u0085b", "", false},
	} {
		got, err := NormalizeRawInput(c.in)
		if c.ok != (err == nil) || got != c.want {
			t.Errorf("NormalizeRawInput(%q) = %q, %v", c.in, got, err)
		}
		var ae *apperr.Error
		if !c.ok && (!errors.As(err, &ae) || ae.Message != RawInputRuleMessage) {
			t.Errorf("NormalizeRawInput(%q) error = %v, want the raw_input message", c.in, err)
		}
	}
}

func TestCurrencyAndSource(t *testing.T) {
	if _, err := ParseCurrency("THB"); err != nil {
		t.Error(err)
	}
	for _, c := range []string{"", "thb", "USD"} {
		if _, err := ParseCurrency(c); apperr.KindOf(err) != apperr.InvalidInput {
			t.Errorf("ParseCurrency(%q) = %v", c, err)
		}
	}
	if s, err := ParseCreateSource("manual"); err != nil || s != SourceManual {
		t.Errorf("ParseCreateSource(manual) = %q, %v", s, err)
	}
	if s, err := ParseCreateSource("text"); err != nil || s != SourceText {
		t.Errorf("ParseCreateSource(text) = %q, %v", s, err)
	}
	for _, s := range []string{"scan", "csv", "", "Manual", "Text", " text"} {
		if _, err := ParseCreateSource(s); err == nil {
			t.Errorf("ParseCreateSource(%q) accepted", s)
		}
	}
	for _, s := range []string{"manual", "text", "scan", "csv"} {
		if _, err := ParseSourceFilter(s); err != nil {
			t.Errorf("ParseSourceFilter(%q) = %v", s, err)
		}
	}
	if _, err := ParseSourceFilter("other"); err == nil {
		t.Error("ParseSourceFilter(other) accepted")
	}
}

func TestParsePeriod(t *testing.T) {
	s := func(v string) *string { return &v }
	show := func(p Period) string {
		var from, to string
		if p.From != nil {
			from = p.From.String()
		}
		if p.To != nil {
			to = p.To.String()
		}
		return from + ".." + to
	}
	for _, c := range []struct {
		name            string
		month, from, to *string
		want            string // "" means refused
	}{
		{"none", nil, nil, nil, ".."},
		{"month", s("2026-02"), nil, nil, "2026-02-01..2026-02-28"},
		{"leap month", s("2024-02"), nil, nil, "2024-02-01..2024-02-29"},
		{"december", s("2025-12"), nil, nil, "2025-12-01..2025-12-31"},
		{"from and to", nil, s("2026-02-09"), s("2026-02-15"), "2026-02-09..2026-02-15"},
		{"one day", nil, s("2026-02-09"), s("2026-02-09"), "2026-02-09..2026-02-09"},
		{"from alone", nil, s("2026-02-09"), nil, "2026-02-09.."},
		{"to alone", nil, nil, s("2026-02-09"), "..2026-02-09"},
		{"from after to", nil, s("2026-02-10"), s("2026-02-09"), ""},
		{"month and from", s("2026-02"), s("2026-02-01"), nil, ""},
		{"month and to", s("2026-02"), nil, s("2026-02-28"), ""},
		{"bad month", s("2026-2"), nil, nil, ""},
		{"month 13", s("2026-13"), nil, nil, ""},
		{"empty month", s(""), nil, nil, ""},
		{"bad from", nil, s("2026-02-30"), nil, ""},
		{"empty to", nil, nil, s(""), ""},
	} {
		p, err := ParsePeriod(c.month, c.from, c.to)
		if c.want == "" {
			if apperr.KindOf(err) != apperr.InvalidInput {
				t.Errorf("%s: %s, %v; want invalid_input", c.name, show(p), err)
			}
			continue
		}
		if err != nil || show(p) != c.want {
			t.Errorf("%s: %s, %v; want %s", c.name, show(p), err, c.want)
		}
	}
}

func TestParseLimit(t *testing.T) {
	s := func(v string) *string { return &v }
	if n, err := ParseLimit(nil); n != DefaultPageSize || err != nil {
		t.Errorf("ParseLimit(nil) = %d, %v", n, err)
	}
	for in, want := range map[string]int{"1": 1, "50": 50, "200": 200, "007": 7} {
		if n, err := ParseLimit(s(in)); n != want || err != nil {
			t.Errorf("ParseLimit(%q) = %d, %v", in, n, err)
		}
	}
	for _, in := range []string{"", "0", "201", "-1", "+5", "1.5", "abc", " 5", "0200", "1e2"} {
		if _, err := ParseLimit(s(in)); apperr.KindOf(err) != apperr.InvalidInput {
			t.Errorf("ParseLimit(%q) = %v; want invalid_input", in, err)
		}
	}
}

func TestCursor(t *testing.T) {
	d, _ := ParseDate("2026-02-14")
	c := Cursor{OccurredOn: d, ID: uuid.Must(uuid.NewV7())}
	got, err := ParseCursor(c.String())
	if err != nil || got != c {
		t.Errorf("round trip: %+v, %v; want %+v", got, err, c)
	}
	if strings.ContainsAny(c.String(), "+/=") {
		t.Errorf("cursor %q is not URL-safe", c.String())
	}
	for _, bad := range []string{
		"", "abc", "%%", c.String() + "=", c.String()[:len(c.String())-1],
		"MjAyNi0wMi0zMF8wMTkyYmQ4Mi0wMDAwLTcwMDAtODAwMC0wMDAwMDAwMDAwMDA", // 2026-02-30_...
		Cursor{OccurredOn: d, ID: uuid.Nil}.String()[:10],
	} {
		if _, err := ParseCursor(bad); apperr.KindOf(err) != apperr.InvalidInput {
			t.Errorf("ParseCursor(%q) = %v; want invalid_input", bad, err)
		}
	}
}
