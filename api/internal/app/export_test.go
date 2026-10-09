package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	exportdomain "github.com/tarlrsk/expense-tracker/api/internal/export/domain"
)

// API-level tests of GET /api/export.csv (PLAN-0004 T5; ADR-0087, ADR-0092).

const exportHeader = "Date,Amount,Kind,Category,Description,Note,Source"

// getExport sends GET /api/export.csv with the raw query string rawQuery ("" for none).
func (e *apiEnv) getExport(tok, rawQuery string) *httptest.ResponseRecorder {
	e.t.Helper()
	path := "/api/export.csv"
	if rawQuery != "" {
		path += "?" + rawQuery
	}
	return e.do(http.MethodGet, path, tok, "")
}

// export returns the lines of GET /api/export.csv?rawQuery after the header, failing the test
// unless it is 200 with the CSV headers, the file name fileName, the byte order mark, the header
// line and CRLF line ends.
func (e *apiEnv) export(tok, rawQuery, fileName string) []string {
	e.t.Helper()
	rec := e.getExport(tok, rawQuery)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("GET /api/export.csv?%s: %d %s", rawQuery, rec.Code, rec.Body.String())
	}
	for name, want := range map[string]string{
		"Content-Type":           "text/csv; charset=utf-8",
		"Content-Disposition":    `attachment; filename="` + fileName + `"`,
		"Cache-Control":          "private, no-store",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := rec.Header().Get(name); got != want {
			e.t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	body, ok := strings.CutPrefix(rec.Body.String(), "\xef\xbb\xbf"+exportHeader+"\r\n")
	if !ok {
		e.t.Fatalf("the file does not start with the byte order mark and the header: %q", rec.Body.String())
	}
	if body == "" {
		return []string{}
	}
	if !strings.HasSuffix(body, "\r\n") {
		e.t.Errorf("the file does not end with CRLF: %q", body)
	}
	return strings.Split(strings.TrimSuffix(body, "\r\n"), "\r\n")
}

// addTx inserts one transaction as the superuser.
func (e *apiEnv) addTx(owner, category uuid.UUID, amount, date, merchant, note, source string) {
	e.t.Helper()
	e.exec(`insert into transactions (owner_id, amount, occurred_on, category_id, merchant, note, source)
		values ($1, $2::numeric, $3::date, $4, $5, $6, $7)`, owner, amount, date, category, merchant, note, source)
}

// newCategory inserts a category as the superuser and returns its id.
func (e *apiEnv) newCategory(owner uuid.UUID, name, kind string, archived bool) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	if err := e.super.QueryRowContext(e.t.Context(),
		`insert into categories (owner_id, name, kind, sort_order, archived) values ($1, $2, $3, 99, $4) returning id`,
		owner, name, kind, archived).Scan(&id); err != nil {
		e.t.Fatalf("insert category %q: %v", name, err)
	}
	return id
}

// exportSeedA gives user a one transaction a day from 2026-09-30 to 2026-11-01, with Thai text,
// income, an archived category, a note with a comma and a line break, and text a spreadsheet
// would run as a formula. It returns the file's lines for every date, oldest first.
func (e *apiEnv) exportSeedA(a account, tok string) []string {
	e.t.Helper()
	food, salary := e.categoryID(tok, "Food"), e.categoryID(tok, "Salary")
	old := e.newCategory(a.id, "Old trips", "expense", true)
	e.addTx(a.id, old, "1234567.89", "2026-09-30", "=Flight", "", "manual")
	e.addTx(a.id, food, "145.00", "2026-10-01", "ข้าวมันไก่", "", "text")
	e.addTx(a.id, salary, "30000.00", "2026-10-15", "Company", "October, after tax\nbonus", "manual")
	e.addTx(a.id, food, "0.50", "2026-10-31", "-5% off", "@home", "manual")
	e.addTx(a.id, food, "60.00", "2026-11-01", "", "", "manual")
	return []string{
		"2026-09-30,1234567.89,expense,Old trips,'=Flight,,manual",
		"2026-10-01,145.00,expense,Food,ข้าวมันไก่,,text",
		"2026-10-15,30000.00,income,Salary,Company,\"October, after tax",
		"bonus\",manual",
		"2026-10-31,0.50,expense,Food,'-5% off,'@home,manual",
		"2026-11-01,60.00,expense,Food,,,manual",
	}
}

func TestExport(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)

	t.Run("401 without a valid token", func(t *testing.T) {
		for _, token := range []string{"", "not-a-token"} {
			rec := e.getExport(token, "")
			if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "unauthenticated" {
				t.Errorf("token %q: %d %s, want 401 unauthenticated", token, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("no transactions: only the header", func(t *testing.T) {
		if got := e.with(t).export(tok, "", "satang-transactions.csv"); len(got) != 0 {
			t.Errorf("lines = %q, want none", got)
		}
	})

	want := e.exportSeedA(a, tok)

	t.Run("every date: the seven columns, oldest first, text made safe", func(t *testing.T) {
		got := e.with(t).export(tok, "", "satang-transactions.csv")
		if !slices.Equal(got, want) {
			t.Errorf("lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		rec := e.getExport(tok, "")
		if strings.Contains(rec.Body.String(), a.id.String()) {
			t.Errorf("the file has the owner id: %s", rec.Body.String())
		}
	})

	t.Run("a period: both ends included, the file named after it", func(t *testing.T) {
		got := e.with(t).export(tok, "from=2026-10-01&to=2026-10-31", "satang-transactions-2026-10-01-to-2026-10-31.csv")
		if w := want[1:5]; !slices.Equal(got, w) {
			t.Errorf("October: lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(w, "\n"))
		}
		if got := e.with(t).export(tok, "from=2026-10-31", "satang-transactions-from-2026-10-31.csv"); !slices.Equal(got, want[4:]) {
			t.Errorf("from 2026-10-31: %q", got)
		}
		if got := e.with(t).export(tok, "to=2026-09-30", "satang-transactions-to-2026-09-30.csv"); !slices.Equal(got, want[:1]) {
			t.Errorf("to 2026-09-30: %q", got)
		}
		if got := e.with(t).export(tok, "from=2027-01-01&to=2027-01-31", "satang-transactions-2027-01-01-to-2027-01-31.csv"); len(got) != 0 {
			t.Errorf("an empty period: %q", got)
		}
	})

	t.Run("bad input is 400 invalid_input as JSON and never quotes the value", func(t *testing.T) {
		for _, c := range []struct {
			query string
			msg   string // "" for the query message
		}{
			{"from=2026-10-02&to=2026-10-01", exportdomain.PeriodMessage},
			{"from=2026-02-30", exportdomain.PeriodMessage},
			{"to=2026-10", exportdomain.PeriodMessage},
			{"from=09%2F10%2F2026", exportdomain.PeriodMessage},
			{"from=%202026-10-01", exportdomain.PeriodMessage},
			{"from=", exportdomain.PeriodMessage},
			{"to", exportdomain.PeriodMessage},
			{"from=2026-10-01&from=2026-10-01", ""},
			{"month=2026-10", ""},
			{"from=2026-10-01&limit=5", ""},
			{"From=2026-10-01", ""},
			{"from=%zz", ""},
		} {
			rec := e.getExport(tok, c.query)
			assertBadRequest(t, c.query, rec, c.msg)
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("%s: Content-Type %q, want JSON", c.query, ct)
			}
			if rec.Header().Get("Content-Disposition") != "" {
				t.Errorf("%s: an error is sent as a download", c.query)
			}
			for _, v := range []string{"2026-02-30", "09/10/2026"} {
				if strings.Contains(rec.Body.String(), v) {
					t.Errorf("%s: the message quotes the value: %s", c.query, rec.Body.String())
				}
			}
		}
	})

	t.Run("a wrong method is 404", func(t *testing.T) {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			rec := e.do(method, "/api/export.csv", tok, "")
			if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
				t.Errorf("%s /api/export.csv: %d %s, want 404 not_found", method, rec.Code, rec.Body.String())
			}
		}
	})
}

// A history longer than a few pages comes back whole: every row once, oldest first.
func TestExportManyPages(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	n := 2*exportdomain.PageSize + 345
	// Amounts 11.00 to n+10.00, at most a few rows a day over about six years.
	e.seedTxs(a.id, e.categoryID(tok, "Food"), n, "manual", "date '2020-01-01' + (g * 7919) % 2200")

	got := e.export(tok, "", "satang-transactions.csv")
	if len(got) != n {
		t.Fatalf("%d lines, want %d", len(got), n)
	}
	seen := make(map[string]bool, n)
	prev := ""
	for _, line := range got {
		f := strings.Split(line, ",")
		if len(f) != 7 || f[2] != "expense" || f[3] != "Food" || f[6] != "manual" {
			t.Fatalf("line %q", line)
		}
		if f[0] < prev {
			t.Fatalf("%s after %s: not oldest first", f[0], prev)
		}
		prev = f[0]
		if seen[f[1]] {
			t.Fatalf("amount %s twice", f[1])
		}
		seen[f[1]] = true
	}
	for i := 1; i <= n; i++ {
		if amount := fmt.Sprintf("%d.00", 10+i); !seen[amount] {
			t.Fatalf("amount %s is missing", amount)
		}
	}
}

// Every user's file holds only their own transactions: the scope is always the caller, and no
// parameter can name another user (ADR-0014, ADR-0092).
func TestExportCrossUser(t *testing.T) {
	e := newAPIEnv(t)
	a, b, c := e.newAccount(accountOpts{}), e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok, cTok := e.freshSession(a.id), e.freshSession(b.id), e.freshSession(c.id)
	wantA := e.exportSeedA(a, aTok)
	zqwx := e.newCategory(b.id, "zqwx-cat", "expense", false)
	e.addTx(b.id, zqwx, "99.99", "2026-10-01", "zqwx", "b-secret-note", "manual")
	e.addTx(b.id, e.categoryID(bTok, "Salary"), "4.44", "2026-10-02", "Makro", "", "manual")
	wantB := []string{
		"2026-10-01,99.99,expense,zqwx-cat,zqwx,b-secret-note,manual",
		"2026-10-02,4.44,income,Salary,Makro,,manual",
	}
	bSecrets := []string{"zqwx", "b-secret-note", "Makro", "99.99", "4.44", b.id.String()}
	aSecrets := []string{"Old trips", "ข้าวมันไก่", "Flight", "1234567.89", "October, after tax", a.id.String()}

	leaks := func(t *testing.T, who, body string, secrets []string) {
		t.Helper()
		for _, s := range secrets {
			if strings.Contains(body, s) {
				t.Errorf("%s's file has %q of another user: %s", who, s, body)
			}
		}
	}

	t.Run("A gets only A's rows, B only B's", func(t *testing.T) {
		if got := e.with(t).export(aTok, "", "satang-transactions.csv"); !slices.Equal(got, wantA) {
			t.Errorf("A: %q, want %q", got, wantA)
		}
		leaks(t, "A", e.getExport(aTok, "").Body.String(), bSecrets)
		if got := e.with(t).export(bTok, "", "satang-transactions.csv"); !slices.Equal(got, wantB) {
			t.Errorf("B: %q, want %q", got, wantB)
		}
		leaks(t, "B", e.getExport(bTok, "").Body.String(), aSecrets)
	})

	t.Run("the same period as B's rows gives A only A's", func(t *testing.T) {
		got := e.with(t).export(aTok, "from=2026-10-01&to=2026-10-02", "satang-transactions-2026-10-01-to-2026-10-02.csv")
		if !slices.Equal(got, wantA[1:2]) {
			t.Errorf("A: %q, want %q", got, wantA[1:2])
		}
	})

	t.Run("an unrelated user gets only the header", func(t *testing.T) {
		if got := e.with(t).export(cTok, "", "satang-transactions.csv"); len(got) != 0 {
			t.Errorf("C: %q", got)
		}
		body := e.getExport(cTok, "").Body.String()
		leaks(t, "C", body, aSecrets)
		leaks(t, "C", body, bSecrets)
	})

	t.Run("no parameter names another user: 400, not B's file", func(t *testing.T) {
		for _, q := range []string{
			"owner_id=" + b.id.String(), "scope=" + b.id.String(), "user_id=" + b.id.String(),
			"from=2026-10-01&owner_id=" + b.id.String(),
		} {
			rec := e.getExport(aTok, q)
			assertBadRequest(t, q, rec, "")
			leaks(t, "A", rec.Body.String(), bSecrets)
		}
	})
}
