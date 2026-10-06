package app

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"

	accountdomain "github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Helpers of the PLAN-0004 T2 tests (GET /api/dashboard, ADR-0088, ADR-0089).

// dashClock is the API's clock in the dashboard tests: 18:00 UTC on 31 October 2026 is already
// 1 November in Bangkok (UTC+7), so the current month is 2026-11 there and 2026-10 in UTC.
var dashClock = time.Date(2026, time.October, 31, 18, 0, 0, 0, time.UTC)

// Exactly the keys of the dashboard body and of its rows.
var (
	dashKeys         = []string{"month", "totals", "categories", "trend", "top_descriptions"}
	dashTotalsKeys   = []string{"spent", "income", "difference"}
	dashCategoryKeys = []string{"category_id", "name", "icon", "archived", "total", "count"}
	dashTrendKeys    = []string{"month", "spent"}
	dashTopKeys      = []string{"description", "total", "count"}
)

type dashTotals struct {
	Spent      string `json:"spent"`
	Income     string `json:"income"`
	Difference string `json:"difference"`
}

type dashCategory struct {
	CategoryID uuid.UUID `json:"category_id"`
	Name       string    `json:"name"`
	Icon       string    `json:"icon"`
	Archived   bool      `json:"archived"`
	Total      string    `json:"total"`
	Count      int       `json:"count"`
}

type dashTrend struct {
	Month string `json:"month"`
	Spent string `json:"spent"`
}

type dashTop struct {
	Description string `json:"description"`
	Total       string `json:"total"`
	Count       int    `json:"count"`
}

// dashBody is the body of GET /api/dashboard.
type dashBody struct {
	Month           string         `json:"month"`
	Totals          dashTotals     `json:"totals"`
	Categories      []dashCategory `json:"categories"`
	Trend           []dashTrend    `json:"trend"`
	TopDescriptions []dashTop      `json:"top_descriptions"`
}

// emptyDash is the body of a month without transactions: zeros, empty lists and six zero trend
// months ending with month.
func emptyDash(month string, trendMonths ...string) dashBody {
	b := dashBody{
		Month: month, Totals: dashTotals{Spent: "0.00", Income: "0.00", Difference: "0.00"},
		Categories: []dashCategory{}, TopDescriptions: []dashTop{},
	}
	for _, m := range trendMonths {
		b.Trend = append(b.Trend, dashTrend{Month: m, Spent: "0.00"})
	}
	return b
}

// getDashboard sends GET /api/dashboard with the raw query string rawQuery ("" for none).
func (e *apiEnv) getDashboard(tok, rawQuery string) *httptest.ResponseRecorder {
	e.t.Helper()
	path := "/api/dashboard"
	if rawQuery != "" {
		path += "?" + rawQuery
	}
	return e.do(http.MethodGet, path, tok, "")
}

// dashboard returns the body of GET /api/dashboard?rawQuery, failing the test unless it is 200
// with exactly the documented keys, at the top and in every row.
func (e *apiEnv) dashboard(tok, rawQuery string) (dashBody, *httptest.ResponseRecorder) {
	e.t.Helper()
	rec := e.getDashboard(tok, rawQuery)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("GET /api/dashboard?%s: %d %s", rawQuery, rec.Code, rec.Body.String())
	}
	m := decodeObject(e.t, rec)
	assertKeys(e.t, "dashboard", m, dashKeys...)
	if totals, ok := m["totals"].(map[string]any); ok {
		assertKeys(e.t, "totals", totals, dashTotalsKeys...)
	} else {
		e.t.Errorf("totals is %#v, want an object", m["totals"])
	}
	for name, keys := range map[string][]string{"categories": dashCategoryKeys, "trend": dashTrendKeys, "top_descriptions": dashTopKeys} {
		rows, ok := m[name].([]any)
		if !ok {
			e.t.Errorf("%s is %#v, want an array", name, m[name])
			continue
		}
		for _, r := range rows {
			row, ok := r.(map[string]any)
			if !ok {
				e.t.Errorf("a row of %s is %#v, want an object", name, r)
				continue
			}
			assertKeys(e.t, name+" row", row, keys...)
		}
	}
	var b dashBody
	decodeStrict(e.t, rec, &b)
	return b, rec
}

// spend inserts one transaction as the superuser; amount is decimal text and date YYYY-MM-DD.
func (e *apiEnv) spend(owner, category uuid.UUID, amount, date, merchant string) {
	e.t.Helper()
	e.exec(`insert into transactions (owner_id, amount, occurred_on, category_id, merchant)
		values ($1, $2::numeric, $3::date, $4, $5)`, owner, amount, date, category, merchant)
}

// clockSession is a session of the user that is fresh at dashClock.
func (e *apiEnv) clockSession(userID uuid.UUID) string {
	return e.newSession(userID, dashClock, dashClock.Add(accountdomain.SessionLifetime))
}
