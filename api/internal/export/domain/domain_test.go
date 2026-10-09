package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	txdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

func ptr(s string) *string { return &s }

func date(t *testing.T, s string) txdomain.Date {
	t.Helper()
	d, ok := txdomain.ParseDate(s)
	if !ok {
		t.Fatalf("not a date: %q", s)
	}
	return d
}

// periodText writes a period as "from..to", with "" for an open end.
func periodText(p Period) string {
	var from, to string
	if p.From != nil {
		from = p.From.String()
	}
	if p.To != nil {
		to = p.To.String()
	}
	return from + ".." + to
}

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		name     string
		from, to *string
		want     string // "" for invalid_input
	}{
		{"nothing is every date", nil, nil, ".."},
		{"both, inclusive", ptr("2026-10-01"), ptr("2026-10-31"), "2026-10-01..2026-10-31"},
		{"one day", ptr("2026-10-09"), ptr("2026-10-09"), "2026-10-09..2026-10-09"},
		{"from alone", ptr("2026-10-01"), nil, "2026-10-01.."},
		{"to alone", nil, ptr("2026-10-31"), "..2026-10-31"},
		{"from after to", ptr("2026-10-02"), ptr("2026-10-01"), ""},
		{"not a real date", ptr("2026-02-30"), nil, ""},
		{"a month", ptr("2026-10"), nil, ""},
		{"empty", nil, ptr(""), ""},
		{"day first", ptr("09/10/2026"), nil, ""},
		{"outer space", ptr(" 2026-10-01"), nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParsePeriod(tt.from, tt.to)
			if tt.want == "" {
				var ae *apperr.Error
				if !errors.As(err, &ae) || ae.Kind != apperr.InvalidInput || ae.Message != PeriodMessage {
					t.Fatalf("error = %v, want invalid_input %q", err, PeriodMessage)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if got := periodText(p); got != tt.want {
				t.Errorf("period = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFileName(t *testing.T) {
	from, to := date(t, "2026-10-01"), date(t, "2026-10-31")
	tests := []struct {
		p    Period
		want string
	}{
		{Period{}, "satang-transactions.csv"},
		{Period{From: &from, To: &to}, "satang-transactions-2026-10-01-to-2026-10-31.csv"},
		{Period{From: &from}, "satang-transactions-from-2026-10-01.csv"},
		{Period{To: &to}, "satang-transactions-to-2026-10-31.csv"},
	}
	for _, tt := range tests {
		if got := FileName(tt.p); got != tt.want {
			t.Errorf("FileName(%s) = %q, want %q", periodText(tt.p), got, tt.want)
		}
	}
}

func TestSafeCell(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"Grab", "Grab"},
		{"=HYPERLINK(\"http://x\")", "'=HYPERLINK(\"http://x\")"},
		{"+66 81 234 5678", "'+66 81 234 5678"},
		{"-50 discount", "'-50 discount"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tcmd", "'\tcmd"},
		{"\rcmd", "'\rcmd"},
		{"a=b", "a=b"},
		{" =1", " =1"},
		{"ข้าวมันไก่", "ข้าวมันไก่"},
		{"'quoted", "'quoted"},
	}
	for _, tt := range tests {
		if got := SafeCell(tt.in); got != tt.want {
			t.Errorf("SafeCell(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFile(t *testing.T) {
	t.Run("only the header when there are no rows", func(t *testing.T) {
		f, err := NewFile()
		if err != nil {
			t.Fatal(err)
		}
		got, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		const want = "\xef\xbb\xbfDate,Amount,Kind,Category,Description,Note,Source\r\n"
		if string(got) != want {
			t.Errorf("file = %q, want %q", got, want)
		}
	})

	t.Run("rows: plain dates and amounts, Thai text, quoting and formula-safe cells", func(t *testing.T) {
		f, err := NewFile()
		if err != nil {
			t.Fatal(err)
		}
		rows := []Row{
			{ID: uuid.New(), OccurredOn: date(t, "2026-10-01"), Amount: "145.00", Kind: "expense", Category: "Food", Description: "ข้าวมันไก่", Source: "text"},
			{ID: uuid.New(), OccurredOn: date(t, "2026-10-02"), Amount: "1234567.89", Kind: "income", Category: "Salary", Note: "October, after tax", Source: "manual"},
			{ID: uuid.New(), OccurredOn: date(t, "2026-10-03"), Amount: "0.50", Kind: "expense", Category: "=Old", Description: `say "hi"`, Note: "line one\nline two", Source: "manual"},
			{ID: uuid.New(), OccurredOn: date(t, "2026-10-04"), Amount: "10.00", Kind: "expense", Category: "Food", Description: "-5% off", Note: "@home", Source: "manual"},
		}
		if err := f.Add(rows[:2]); err != nil {
			t.Fatal(err)
		}
		if err := f.Add(rows[2:]); err != nil {
			t.Fatal(err)
		}
		got, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		want := "\xef\xbb\xbf" +
			"Date,Amount,Kind,Category,Description,Note,Source\r\n" +
			"2026-10-01,145.00,expense,Food,ข้าวมันไก่,,text\r\n" +
			"2026-10-02,1234567.89,income,Salary,,\"October, after tax\",manual\r\n" +
			"2026-10-03,0.50,expense,'=Old,\"say \"\"hi\"\"\",\"line one\r\nline two\",manual\r\n" +
			"2026-10-04,10.00,expense,Food,'-5% off,'@home,manual\r\n"
		if string(got) != want {
			t.Errorf("file =\n%q\nwant\n%q", got, want)
		}
	})
}
