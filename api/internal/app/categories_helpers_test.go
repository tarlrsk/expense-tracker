package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Helpers of the PLAN-0002 T7 tests (/api/categories, ADR-0069).

// defaultCategoryNames are the seeded categories in their seeded order (migration 0003).
var defaultCategoryNames = []string{
	"Food", "Groceries", "Transport", "Bills & Utilities", "Shopping", "Health", "Education",
	"Family support", "Donations / Tamboon", "Entertainment", "Other", "Salary", "Other income",
}

// defaultCategoryIcons are the seeded icons, in the same order (ADR-0072).
var defaultCategoryIcons = []string{
	"🍜", "🛒", "🚌", "💡", "🛍️", "💊", "🎓", "👪", "🙏", "🎬", "📦", "💼", "💰",
}

// categoryItemKeys are exactly the keys of a category in a response.
var categoryItemKeys = []string{"id", "name", "icon", "kind", "archived", "sort_order", "created_at", "updated_at"}

// category is one category of a response.
type category struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Kind      string    `json:"kind"`
	Archived  bool      `json:"archived"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type categoryList struct {
	Categories []category `json:"categories"`
}

// decodeStrict decodes a JSON response into dst, refusing unknown fields (owner_id, for one).
func decodeStrict(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
}

// listCategories returns the caller's list, failing the test unless GET answers 200.
func (e *apiEnv) listCategories(tok string) []category {
	e.t.Helper()
	rec := e.do(http.MethodGet, "/api/categories", tok, "")
	if rec.Code != http.StatusOK {
		e.t.Fatalf("GET /api/categories: %d %s", rec.Code, rec.Body.String())
	}
	var l categoryList
	decodeStrict(e.t, rec, &l)
	if l.Categories == nil {
		e.t.Fatalf("GET /api/categories: categories is null: %s", rec.Body.String())
	}
	return l.Categories
}

func (e *apiEnv) createCategory(tok, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/categories", tok, body)
}

// mustCreateCategory creates an expense category and returns it, failing the test unless 201.
func (e *apiEnv) mustCreateCategory(tok, name string) category {
	e.t.Helper()
	rec := e.createCategory(tok, jsonBody(e.t, map[string]string{"name": name, "kind": "expense"}))
	if rec.Code != http.StatusCreated {
		e.t.Fatalf("create %q: %d %s", name, rec.Code, rec.Body.String())
	}
	var c category
	decodeStrict(e.t, rec, &c)
	return c
}

func (e *apiEnv) patchCategory(tok, id, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPatch, "/api/categories/"+id, tok, body)
}

// mustPatchCategory patches and returns the category, failing the test unless 200.
func (e *apiEnv) mustPatchCategory(tok string, id uuid.UUID, body string) category {
	e.t.Helper()
	rec := e.patchCategory(tok, id.String(), body)
	if rec.Code != http.StatusOK {
		e.t.Fatalf("PATCH %s %s: %d %s", id, body, rec.Code, rec.Body.String())
	}
	var c category
	decodeStrict(e.t, rec, &c)
	return c
}

func (e *apiEnv) reorder(tok string, ids []uuid.UUID) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPut, "/api/categories/order", tok, jsonBody(e.t, map[string][]uuid.UUID{"ids": ids}))
}

// categoryRows is every stored column of the user's categories, read as the superuser, ordered
// by id: two equal snapshots mean nothing was written (updated_at moves on every update).
func (e *apiEnv) categoryRows(userID uuid.UUID) []string {
	e.t.Helper()
	rows, err := e.super.QueryContext(e.t.Context(),
		`select id, owner_id, name, icon, kind, archived, sort_order, created_at, updated_at
		 from categories where owner_id = $1 order by id`, userID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var (
			id, owner          uuid.UUID
			name, icon, kind   string
			archived           bool
			sortOrder          int
			created, updatedAt time.Time
		)
		if err := rows.Scan(&id, &owner, &name, &icon, &kind, &archived, &sortOrder, &created, &updatedAt); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, fmt.Sprint(id, owner, name, icon, kind, archived, sortOrder, created.UnixMicro(), updatedAt.UnixMicro()))
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// addCategories inserts n archived and active categories for the user as the superuser, after
// the defaults.
func (e *apiEnv) addCategories(userID uuid.UUID, n int) {
	e.t.Helper()
	e.exec(`insert into categories (owner_id, name, kind, sort_order, archived)
		select $1, 'Extra ' || g, 'expense', 100 + g, g % 2 = 0 from generate_series(1, $2::int) g`, userID, n)
}

// activeIDs returns the ids of the non-archived categories of list, in list order.
func activeIDs(list []category) []uuid.UUID {
	var ids []uuid.UUID
	for _, c := range list {
		if !c.Archived {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// byName returns the category of list named name, failing the test when there is none.
func byName(t *testing.T, list []category, name string) category {
	t.Helper()
	i := slices.IndexFunc(list, func(c category) bool { return c.Name == name })
	if i < 0 {
		t.Fatalf("no category %q in %v", name, list)
	}
	return list[i]
}

// assertOrdered fails unless list is ordered by sort_order, then id.
func assertOrdered(t *testing.T, list []category) {
	t.Helper()
	sorted := slices.IsSortedFunc(list, func(a, b category) int {
		if a.SortOrder != b.SortOrder {
			return a.SortOrder - b.SortOrder
		}
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	if !sorted {
		t.Errorf("list is not ordered by sort_order, then id: %v", list)
	}
}
