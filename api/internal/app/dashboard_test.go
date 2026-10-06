package app

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	dashboarddomain "github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
)

// API-level tests of GET /api/dashboard (PLAN-0004 T2; ADR-0042, ADR-0088, ADR-0089).

// dashSeedA gives user a, in 2026-02 and around it, spending in three default categories and in
// an archived one, income, an empty description and one description in three spellings. It
// returns the body expected for 2026-02.
func (e *apiEnv) dashSeedA(a account, tok string) dashBody {
	e.t.Helper()
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")
	shopping, salary := e.categoryID(tok, "Shopping"), e.categoryID(tok, "Salary")
	pets := e.mustCreateCategory(tok, "Pets")

	// 2026-02, from its first to its last day.
	e.spend(a.id, salary, "500.00", "2026-02-01", "Salary")
	e.spend(a.id, food, "120.50", "2026-02-03", "Grab")
	e.spend(a.id, transport, "300.00", "2026-02-05", "Grab")
	e.spend(a.id, pets.ID, "400.00", "2026-02-07", "Vet")
	e.spend(a.id, food, "79.50", "2026-02-10", "grab")
	e.spend(a.id, shopping, "5.00", "2026-02-20", "")
	e.spend(a.id, food, "50.00", "2026-02-28", "Lunch")
	// The trend months before it; income never counts as spending.
	e.spend(a.id, food, "99.99", "2025-11-15", "Grab")
	e.spend(a.id, food, "10.01", "2025-12-31", "Lunch")
	e.spend(a.id, transport, "1.00", "2026-01-31", "Vet")
	e.spend(a.id, salary, "2000.00", "2026-01-31", "Salary")
	// Outside the trend and the month.
	e.spend(a.id, food, "7.00", "2025-08-31", "Grab")
	e.spend(a.id, food, "8.00", "2026-03-01", "Grab")
	e.spend(a.id, salary, "9999.00", "2026-03-01", "Salary")
	e.mustPatchCategory(tok, pets.ID, `{"archived":true}`)

	return dashBody{
		Month:  "2026-02",
		Totals: dashTotals{Spent: "955.00", Income: "500.00", Difference: "-455.00"},
		Categories: []dashCategory{
			{CategoryID: pets.ID, Name: "Pets", Icon: "", Archived: true, Total: "400.00", Count: 1},
			{CategoryID: transport, Name: "Transport", Icon: "🚌", Total: "300.00", Count: 1},
			{CategoryID: food, Name: "Food", Icon: "🍜", Total: "250.00", Count: 3},
			{CategoryID: shopping, Name: "Shopping", Icon: "🛍️", Total: "5.00", Count: 1},
		},
		Trend: []dashTrend{
			{"2025-09", "0.00"}, {"2025-10", "0.00"}, {"2025-11", "99.99"}, {"2025-12", "10.01"}, {"2026-01", "1.00"},
			{"2026-02", "955.00"},
		},
		TopDescriptions: []dashTop{
			{Description: "Grab", Total: "500.00", Count: 3},
			{Description: "Vet", Total: "400.00", Count: 1},
			{Description: "Lunch", Total: "50.00", Count: 1},
		},
	}
}

// dashSeedB gives user b other spending in the same month, and returns the body expected for
// 2026-02.
func (e *apiEnv) dashSeedB(b account, tok string) dashBody {
	e.t.Helper()
	food, groceries, other := e.categoryID(tok, "Food"), e.categoryID(tok, "Groceries"), e.categoryID(tok, "Other income")
	e.spend(b.id, food, "33.33", "2026-02-14", "B secret zqwx")
	e.spend(b.id, groceries, "66.67", "2026-02-15", "Makro")
	e.spend(b.id, other, "10.00", "2026-02-16", "zqwx refund")
	e.spend(b.id, food, "4.44", "2025-12-01", "zqwx")
	return dashBody{
		Month:  "2026-02",
		Totals: dashTotals{Spent: "100.00", Income: "10.00", Difference: "-90.00"},
		Categories: []dashCategory{
			{CategoryID: groceries, Name: "Groceries", Icon: "🛒", Total: "66.67", Count: 1},
			{CategoryID: food, Name: "Food", Icon: "🍜", Total: "33.33", Count: 1},
		},
		Trend: []dashTrend{
			{"2025-09", "0.00"}, {"2025-10", "0.00"}, {"2025-11", "0.00"}, {"2025-12", "4.44"}, {"2026-01", "0.00"},
			{"2026-02", "100.00"},
		},
		TopDescriptions: []dashTop{
			{Description: "Makro", Total: "66.67", Count: 1},
			{Description: "B secret zqwx", Total: "33.33", Count: 1},
		},
	}
}

func assertDash(t *testing.T, what string, got, want dashBody) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got %+v\nwant %+v", what, got, want)
	}
}

func TestDashboard(t *testing.T) {
	e := newAPIEnv(t, withClock(dashClock))
	a := e.newAccount(accountOpts{})
	tok := e.clockSession(a.id)
	want := e.dashSeedA(a, tok)

	t.Run("401 without a valid token", func(t *testing.T) {
		for _, token := range []string{"", "not-a-token"} {
			rec := e.getDashboard(token, "month=2026-02")
			if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "unauthenticated" {
				t.Errorf("token %q: %d %s, want 401 unauthenticated", token, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("a seeded month: totals, breakdown, trend across the year and top descriptions", func(t *testing.T) {
		got, rec := e.with(t).dashboard(tok, "month=2026-02")
		assertDash(t, "2026-02", got, want)
		if cc := rec.Header().Get("Cache-Control"); cc != "private, no-store" {
			t.Errorf("Cache-Control = %q, want %q", cc, "private, no-store")
		}
		if strings.Contains(rec.Body.String(), a.id.String()) {
			t.Errorf("the body has the owner id: %s", rec.Body.String())
		}
	})

	t.Run("a month without transactions: zeros, [] lists, six zero trend months", func(t *testing.T) {
		got, rec := e.with(t).dashboard(tok, "month=2025-06")
		assertDash(t, "2025-06", got, emptyDash("2025-06", "2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"))
		body := rec.Body.String()
		if !strings.Contains(body, `"categories":[]`) || !strings.Contains(body, `"top_descriptions":[]`) || strings.Contains(body, "null") {
			t.Errorf("empty lists are not []: %s", body)
		}
	})

	t.Run("no month: the current month in the app time zone", func(t *testing.T) {
		got, _ := e.with(t).dashboard(tok, "")
		if got.Month != "2026-11" || got.Trend[len(got.Trend)-1].Month != "2026-11" {
			t.Errorf("default month %q, trend %+v; want 2026-11 (Bangkok)", got.Month, got.Trend)
		}
		utc := newAPIEnv(t, withClock(dashClock), withZone(time.UTC))
		if got, _ := utc.dashboard(utc.clockSession(a.id), ""); got.Month != "2026-10" {
			t.Errorf("default month in UTC %q, want 2026-10", got.Month)
		}
	})

	t.Run("the ends of the range are accepted", func(t *testing.T) {
		got, _ := e.with(t).dashboard(tok, "month=2000-01")
		assertDash(t, "2000-01", got, emptyDash("2000-01", "1999-08", "1999-09", "1999-10", "1999-11", "1999-12", "2000-01"))
		got, _ = e.with(t).dashboard(tok, "month=2027-11")
		assertDash(t, "2027-11", got, emptyDash("2027-11", "2027-06", "2027-07", "2027-08", "2027-09", "2027-10", "2027-11"))
	})

	t.Run("bad input is 400 invalid_input and never quotes the value", func(t *testing.T) {
		for _, c := range []struct {
			query string
			msg   string // "" for the query message
		}{
			{"month=2026-13", dashboarddomain.MonthMessage},
			{"month=2026-00", dashboarddomain.MonthMessage},
			{"month=2026-1", dashboarddomain.MonthMessage},
			{"month=26-10", dashboarddomain.MonthMessage},
			{"month=2026-10-01", dashboarddomain.MonthMessage},
			{"month=2026%2F10", dashboarddomain.MonthMessage},
			{"month=%202026-10", dashboarddomain.MonthMessage},
			{"month=2026-10%20", dashboarddomain.MonthMessage},
			{"month=" + url.QueryEscape("２０２６-１０"), dashboarddomain.MonthMessage},
			{"month=1999-12", dashboarddomain.MonthMessage},
			{"month=2027-12", dashboarddomain.MonthMessage},
			{"month=", dashboarddomain.MonthMessage},
			{"month", dashboarddomain.MonthMessage},
			{"month=2026-02&month=2026-02", ""},
			{"month=2026-02&month=2026-03", ""},
			{"from=2026-02-01", ""},
			{"month=2026-02&limit=5", ""},
			{"month=%zz", ""},
		} {
			rec := e.getDashboard(tok, c.query)
			assertBadRequest(t, c.query, rec, c.msg)
			for _, v := range []string{"2026-13", "1999-12", "2027-12", "2026-10-01"} {
				if strings.Contains(c.query, v) && strings.Contains(rec.Body.String(), v) {
					t.Errorf("%s: the message quotes the value: %s", c.query, rec.Body.String())
				}
			}
		}
	})

	t.Run("a wrong method is 404", func(t *testing.T) {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			rec := e.do(method, "/api/dashboard", tok, "")
			if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
				t.Errorf("%s /api/dashboard: %d %s, want 404 not_found", method, rec.Code, rec.Body.String())
			}
		}
	})
}

// Every user sees only totals of their own data: the scope is always the caller, and no parameter
// can name another user (ADR-0014, ADR-0088).
func TestDashboardCrossUser(t *testing.T) {
	e := newAPIEnv(t, withClock(dashClock))
	a, b, c := e.newAccount(accountOpts{}), e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok, cTok := e.clockSession(a.id), e.clockSession(b.id), e.clockSession(c.id)
	wantA, wantB := e.dashSeedA(a, aTok), e.dashSeedB(b, bTok)

	// leaks fails when body holds any of the other user's ids or descriptions.
	leaks := func(t *testing.T, who, body string, other account, otherTok string, words ...string) {
		t.Helper()
		secrets := append([]string{other.id.String()}, words...)
		for _, cat := range e.with(t).listCategories(otherTok) {
			secrets = append(secrets, cat.ID.String())
		}
		for _, s := range secrets {
			if strings.Contains(body, s) {
				t.Errorf("%s's dashboard has %q of the other user: %s", who, s, body)
			}
		}
	}

	t.Run("A sees only A's totals, B only B's, in the same month", func(t *testing.T) {
		gotA, recA := e.with(t).dashboard(aTok, "month=2026-02")
		assertDash(t, "A", gotA, wantA)
		leaks(t, "A", recA.Body.String(), b, bTok, "zqwx", "Makro", "33.33", "66.67", "4.44")
		gotB, recB := e.with(t).dashboard(bTok, "month=2026-02")
		assertDash(t, "B", gotB, wantB)
		leaks(t, "B", recB.Body.String(), a, aTok, "Grab", "Vet", "Lunch", "Pets", "955.00", "99.99")
	})

	t.Run("an unrelated user gets zeros and empty lists", func(t *testing.T) {
		got, rec := e.with(t).dashboard(cTok, "month=2026-02")
		assertDash(t, "C", got, emptyDash("2026-02", "2025-09", "2025-10", "2025-11", "2025-12", "2026-01", "2026-02"))
		leaks(t, "C", rec.Body.String(), a, aTok, "Grab", "Vet")
		leaks(t, "C", rec.Body.String(), b, bTok, "zqwx", "Makro")
	})

	t.Run("no parameter names another user: 400, not B's data", func(t *testing.T) {
		for _, q := range []string{
			"owner_id=" + b.id.String(), "scope=" + b.id.String(), "user_id=" + b.id.String(),
			"month=2026-02&owner_id=" + b.id.String(), "month=2026-02&scope=" + b.id.String(),
		} {
			rec := e.getDashboard(aTok, q)
			assertBadRequest(t, q, rec, "")
			if strings.Contains(rec.Body.String(), "zqwx") || strings.Contains(rec.Body.String(), b.id.String()) {
				t.Errorf("%s: %s", q, rec.Body.String())
			}
		}
	})

	t.Run("A's dashboard is unchanged after B's requests", func(t *testing.T) {
		got, _ := e.with(t).dashboard(aTok, "month=2026-02")
		assertDash(t, "A again", got, wantA)
	})
}
