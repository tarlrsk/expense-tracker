package app

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	// The API binary embeds the zone database (cmd/api); so do these tests.
	_ "time/tzdata"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Helpers of the PLAN-0002 T8 tests (/api/transactions, ADR-0040, ADR-0041, ADR-0042, ADR-0071).

// testZone is APP_TIME_ZONE in API tests: the default, Asia/Bangkok (UTC+7).
func testZone(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		t.Fatalf("load Asia/Bangkok: %v", err)
	}
	return loc
}

// withClock makes the API's clock always say now.
func withClock(now time.Time) func(*registry.Deps) {
	return func(d *registry.Deps) { d.Clock = func() time.Time { return now } }
}

// withZone sets the API's app time zone.
func withZone(loc *time.Location) func(*registry.Deps) {
	return func(d *registry.Deps) { d.Config.AppTimeZone = loc }
}

// txItemKeys are exactly the keys of a transaction in a response.
var txItemKeys = []string{
	"id", "owner_id", "amount", "currency", "occurred_on", "merchant", "category_id", "note", "source", "raw_input", "created_at",
	"updated_at",
}

// txItem is one transaction of a response.
type txItem struct {
	ID         uuid.UUID `json:"id"`
	OwnerID    uuid.UUID `json:"owner_id"`
	Amount     string    `json:"amount"`
	Currency   string    `json:"currency"`
	OccurredOn string    `json:"occurred_on"`
	Merchant   string    `json:"merchant"`
	CategoryID uuid.UUID `json:"category_id"`
	Note       string    `json:"note"`
	Source     string    `json:"source"`
	RawInput   string    `json:"raw_input"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type txList struct {
	Transactions []txItem `json:"transactions"`
	NextCursor   *string  `json:"next_cursor"`
}

// newTxID is a fresh UUID version 7, like a client makes.
func newTxID(t *testing.T) string {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id.String()
}

// txBody is a valid create body for category with a new id; fields override or add keys, and a
// nil value removes the key.
func txBody(t *testing.T, category uuid.UUID, fields map[string]any) string {
	t.Helper()
	body := map[string]any{"id": newTxID(t), "amount": "100.00", "occurred_on": "2026-01-15", "category_id": category.String()}
	for k, v := range fields {
		if v == nil {
			delete(body, k)
			continue
		}
		body[k] = v
	}
	return jsonBody(t, body)
}

func (e *apiEnv) createTx(tok, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/transactions", tok, body)
}

// mustCreateTx creates a transaction from txBody and returns it, failing the test unless 201.
func (e *apiEnv) mustCreateTx(tok string, category uuid.UUID, fields map[string]any) txItem {
	e.t.Helper()
	rec := e.createTx(tok, txBody(e.t, category, fields))
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("create transaction %v: %d %s", fields, rec.Code, rec.Body.String())
	}
	var it txItem
	decodeStrict(e.t, rec, &it)
	return it
}

func (e *apiEnv) patchTx(tok, id, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPatch, "/api/transactions/"+id, tok, body)
}

// mustPatchTx patches and returns the transaction, failing the test unless 200.
func (e *apiEnv) mustPatchTx(tok string, id uuid.UUID, body string) txItem {
	e.t.Helper()
	rec := e.patchTx(tok, id.String(), body)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("PATCH %s %s: %d %s", id, body, rec.Code, rec.Body.String())
	}
	var it txItem
	decodeStrict(e.t, rec, &it)
	return it
}

func (e *apiEnv) deleteTx(tok, id string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodDelete, "/api/transactions/"+id, tok, "")
}

func (e *apiEnv) getTxs(tok string, query url.Values) *httptest.ResponseRecorder {
	e.t.Helper()
	path := "/api/transactions"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	return e.do(http.MethodGet, path, tok, "")
}

// listTxs returns one page, failing the test unless GET answers 200.
func (e *apiEnv) listTxs(tok string, query url.Values) txList {
	e.t.Helper()
	rec := e.getTxs(tok, query)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("GET /api/transactions?%s: %d %s", query.Encode(), rec.Code, rec.Body.String())
	}
	var l txList
	decodeStrict(e.t, rec, &l)
	if l.Transactions == nil {
		e.t.Fatalf("transactions is null: %s", rec.Body.String())
	}
	return l
}

// listAllTxs follows next_cursor from the first page with the given query and page size, and
// returns every transaction and the sizes of the pages.
func (e *apiEnv) listAllTxs(tok string, query url.Values, limit int) (all []txItem, sizes []int) {
	e.t.Helper()
	q := maps.Clone(query)
	if q == nil {
		q = url.Values{}
	}
	q.Set("limit", fmt.Sprint(limit))
	for range 1000 {
		page := e.listTxs(tok, q)
		all = append(all, page.Transactions...)
		sizes = append(sizes, len(page.Transactions))
		if page.NextCursor == nil {
			return all, sizes
		}
		q.Set("cursor", *page.NextCursor)
	}
	e.t.Fatal("paging did not end")
	return nil, nil
}

// query builds url.Values from name, value pairs.
func query(pairs ...string) url.Values {
	q := url.Values{}
	for len(pairs) >= 2 {
		q.Add(pairs[0], pairs[1])
		pairs = pairs[2:]
	}
	return q
}

// txRows is every stored column of the user's transactions, read as the superuser, ordered by
// id: two equal snapshots mean nothing was written (updated_at moves on every update).
func (e *apiEnv) txRows(userID uuid.UUID) []string {
	e.t.Helper()
	rows, err := e.super.QueryContext(e.t.Context(),
		`select id, owner_id, amount::text, currency, occurred_on::text, merchant, category_id, note, source, raw_input,
		        created_at, updated_at
		 from transactions where owner_id = $1 order by id`, userID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var (
			id, owner, category                                 uuid.UUID
			amount, currency, date, merchant, note, source, raw string
			created, updatedAt                                  time.Time
		)
		if err := rows.Scan(&id, &owner, &amount, &currency, &date, &merchant, &category, &note, &source, &raw, &created, &updatedAt); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, fmt.Sprint(id, owner, amount, currency, date, merchant, category, note, source, raw,
			created.UnixMicro(), updatedAt.UnixMicro()))
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// seedTxs inserts n transactions of the user in category as the superuser, with the given source,
// and dates from dateSQL, an SQL expression of g (1 to n). It returns nothing: tests read the
// rows back through the API or txRows.
func (e *apiEnv) seedTxs(userID, category uuid.UUID, n int, source, dateSQL string) {
	e.t.Helper()
	e.exec(`insert into transactions (owner_id, amount, occurred_on, category_id, source)
		select $1, 10 + g, (`+dateSQL+`)::date, $2, $3 from generate_series(1, $4::int) g`, userID, category, source, n)
}

// categoryID returns the id of the user's category named name.
func (e *apiEnv) categoryID(tok, name string) uuid.UUID {
	e.t.Helper()
	return byName(e.t, e.listCategories(tok), name).ID
}

// txIDs returns the ids of items, in order.
func txIDs(items []txItem) []uuid.UUID {
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}
