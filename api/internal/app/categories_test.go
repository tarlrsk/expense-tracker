package app

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	accountdomain "github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// API-level tests of /api/categories (PLAN-0002 T7; ADR-0039, ADR-0061, ADR-0069).

func TestListCategories(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)

	rec := e.do(http.MethodGet, "/api/categories", tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeObject(t, rec)
	assertKeys(t, "list", body, "categories")
	items, _ := body["categories"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		assertKeys(t, "category", m, categoryItemKeys...)
	}

	list := e.listCategories(tok)
	if len(list) != defaultCategories {
		t.Fatalf("categories = %d, want %d", len(list), defaultCategories)
	}
	for i, c := range list {
		wantKind := "expense"
		if c.Name == "Salary" || c.Name == "Other income" {
			wantKind = "income"
		}
		if c.Name != defaultCategoryNames[i] || c.SortOrder != i+1 || c.Kind != wantKind || c.Archived || c.Icon != defaultCategoryIcons[i] {
			t.Errorf("category %d = %+v, want %q %q with sort_order %d", i, c, defaultCategoryNames[i], defaultCategoryIcons[i], i+1)
		}
		if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() || c.ID == uuid.Nil {
			t.Errorf("category %d has a zero field: %+v", i, c)
		}
	}
}

func TestCreateCategory(t *testing.T) {
	e := newAPIEnv(t)
	user := func(e *apiEnv) string { return e.freshSession(e.newAccount(accountOpts{}).id) }

	t.Run("goes last and is returned whole", func(t *testing.T) {
		e := e.with(t)
		tok := user(e)
		rec := e.createCategory(tok, `{"name":"Pets","kind":"expense","icon":"🐶"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
		assertKeys(t, "category", decodeObject(t, rec), categoryItemKeys...)
		var c category
		decodeStrict(t, rec, &c)
		if c.Name != "Pets" || c.Kind != "expense" || c.Icon != "🐶" || c.Archived || c.SortOrder != defaultCategories+1 {
			t.Errorf("created %+v", c)
		}
		list := e.listCategories(tok)
		if last := list[len(list)-1]; last != c {
			t.Errorf("last in the list = %+v, want %+v", last, c)
		}
		income := e.mustCreateCategory(tok, "Bonus")
		if income.SortOrder != defaultCategories+2 {
			t.Errorf("second new category has sort_order %d", income.SortOrder)
		}
	})

	t.Run("after archived categories with higher numbers it still goes last", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		e.exec("update categories set archived = true, sort_order = 40 where owner_id = $1 and name = 'Food'", a.id)
		if c := e.mustCreateCategory(tok, "Pets"); c.SortOrder != 41 {
			t.Errorf("sort_order = %d, want 41", c.SortOrder)
		}
	})

	t.Run("name: whitespace normalised, 1 to 50 characters", func(t *testing.T) {
		e := e.with(t)
		tok := user(e)
		for _, c := range []struct{ in, want string }{
			{"  Pet \t\n food  ", "Pet food"},
			{"x", "x"},
			{strings.Repeat("ก", 50), strings.Repeat("ก", 50)},
			{"\u00a0Gifts\u3000for  kids ", "Gifts for kids"},
		} {
			got := e.mustCreateCategory(tok, c.in)
			if got.Name != c.want {
				t.Errorf("create %q: name %q, want %q", c.in, got.Name, c.want)
			}
		}
		for _, name := range []string{"", "   ", "\t\n", strings.Repeat("ก", 51), "Bad\x00name", "Bad\x1bname"} {
			rec := e.createCategory(tok, jsonBody(t, map[string]string{"name": name, "kind": "expense"}))
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" ||
				!strings.Contains(rec.Body.String(), domain.NameRuleMessage) {
				t.Errorf("create %q: %d %s, want 400 invalid_input", name, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("kind must be expense or income", func(t *testing.T) {
		e := e.with(t)
		tok := user(e)
		for _, body := range []string{
			`{"name":"Pets","kind":"transfer"}`, `{"name":"Pets","kind":"Expense"}`, `{"name":"Pets","kind":""}`,
			`{"name":"Pets"}`, `{"name":"Pets","kind":1}`,
		} {
			rec := e.createCategory(tok, body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("create %s: %d %s, want 400", body, rec.Code, rec.Body.String())
			}
		}
		rec := e.createCategory(tok, `{"name":"Bonus","kind":"income"}`)
		var c category
		decodeStrict(t, rec, &c)
		if rec.Code != http.StatusCreated || c.Kind != "income" {
			t.Errorf("income: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("icon: optional, trimmed, at most 32 characters", func(t *testing.T) {
		e := e.with(t)
		tok := user(e)
		for i, c := range []struct{ icon, want string }{
			{" 🐶 ", "🐶"}, {"", ""}, {strings.Repeat("ก", 32), strings.Repeat("ก", 32)}, {"   ", ""},
		} {
			rec := e.createCategory(tok, jsonBody(t, map[string]string{"name": "Icon " + string(rune('a'+i)), "kind": "expense", "icon": c.icon}))
			var got category
			decodeStrict(t, rec, &got)
			if rec.Code != http.StatusCreated || got.Icon != c.want {
				t.Errorf("icon %q: %d %s, want %q", c.icon, rec.Code, rec.Body.String(), c.want)
			}
		}
		rec := e.createCategory(tok, jsonBody(t, map[string]string{"name": "Long icon", "kind": "expense", "icon": strings.Repeat("ก", 33)}))
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
			t.Errorf("33-character icon: %d %s, want 400", rec.Code, rec.Body.String())
		}
		if got := e.mustCreateCategory(tok, "No icon"); got.Icon != "" {
			t.Errorf("missing icon gave %q", got.Icon)
		}
	})

	t.Run("a taken name, ignoring case, is a conflict; one used only by an archived category is free", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		before := e.categoryRows(a.id)
		for _, name := range []string{"Food", "food", " FOOD ", "bills  &  utilities"} {
			rec := e.createCategory(tok, jsonBody(t, map[string]string{"name": name, "kind": "income"}))
			if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" ||
				!strings.Contains(rec.Body.String(), domain.NameTakenMessage) {
				t.Errorf("create %q: %d %s, want 409", name, rec.Code, rec.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused create changed the categories")
		}
		shopping := byName(t, e.listCategories(tok), "Shopping")
		e.mustPatchCategory(tok, shopping.ID, `{"archived":true}`)
		if c := e.mustCreateCategory(tok, "shopping"); c.Archived || c.ID == shopping.ID {
			t.Errorf("created %+v", c)
		}
	})

	t.Run("two creates of one name at the same moment: one 201, one 409", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		codes := concurrently(
			func() int { return e.createCategory(tok, `{"name":"Pets","kind":"expense"}`).Code },
			func() int { return e.createCategory(tok, `{"name":"PETS","kind":"expense"}`).Code },
		)
		if !slices.Equal(codes, []int{http.StatusCreated, http.StatusConflict}) {
			t.Errorf("statuses = %v, want 201 and 409", codes)
		}
		if n := e.count("select count(*) from categories where owner_id = $1 and lower(name) = 'pets'", a.id); n != 1 {
			t.Errorf("categories named pets = %d, want 1", n)
		}
	})

	t.Run("at most 200 categories, archived ones included", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		e.addCategories(a.id, domain.MaxCategories-defaultCategories-1)
		if c := e.mustCreateCategory(tok, "The 200th"); c.SortOrder == 0 {
			t.Errorf("created %+v", c)
		}
		before := e.categoryRows(a.id)
		rec := e.createCategory(tok, `{"name":"The 201st","kind":"expense"}`)
		if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" ||
			!strings.Contains(rec.Body.String(), domain.LimitMessage) {
			t.Errorf("201st: %d %s, want 409 with the limit message", rec.Code, rec.Body.String())
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("the refused create changed the categories")
		}
	})

	t.Run("two creates at 199 at the same moment: one 201, one 409", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		e.addCategories(a.id, domain.MaxCategories-defaultCategories-1)
		codes := concurrently(
			func() int { return e.createCategory(tok, `{"name":"Last A","kind":"expense"}`).Code },
			func() int { return e.createCategory(tok, `{"name":"Last B","kind":"expense"}`).Code },
		)
		if !slices.Equal(codes, []int{http.StatusCreated, http.StatusConflict}) {
			t.Errorf("statuses = %v, want 201 and 409", codes)
		}
		if n := e.count("select count(*) from categories where owner_id = $1", a.id); n != domain.MaxCategories {
			t.Errorf("categories = %d, want %d", n, domain.MaxCategories)
		}
	})

	t.Run("malformed bodies and unknown fields are refused", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		before := e.categoryRows(a.id)
		for _, body := range []string{
			``, `{`, `[]`, `{"name":5,"kind":"expense"}`,
			`{"name":"Pets","kind":"expense","sort_order":1}`,
			`{"name":"Pets","kind":"expense","archived":true}`,
			`{"name":"Pets","kind":"expense","id":"` + uuid.NewString() + `"}`,
			`{"name":"Pets","kind":"expense"} {}`,
		} {
			rec := e.createCategory(tok, body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("create %s: %d %s, want 400", body, rec.Code, rec.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused create changed the categories")
		}
	})
}

func TestUpdateCategory(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	list := e.listCategories(tok)
	food, groceries := byName(t, list, "Food"), byName(t, list, "Groceries")

	t.Run("rename, normalised; unchanged fields stay", func(t *testing.T) {
		got := e.mustPatchCategory(tok, food.ID, `{"name":"  Eating \t out "}`)
		if got.Name != "Eating out" || got.ID != food.ID || got.Kind != food.Kind || got.SortOrder != food.SortOrder ||
			got.Icon != food.Icon || got.Archived || !got.CreatedAt.Equal(food.CreatedAt) || !got.UpdatedAt.After(food.UpdatedAt) {
			t.Errorf("after rename %+v, before %+v", got, food)
		}
		if byName(t, e.listCategories(tok), "Eating out").ID != food.ID {
			t.Error("the list does not show the new name")
		}
	})

	t.Run("its own name in another letter case works", func(t *testing.T) {
		if got := e.mustPatchCategory(tok, food.ID, `{"name":"EATING OUT"}`); got.Name != "EATING OUT" {
			t.Errorf("name = %q", got.Name)
		}
	})

	t.Run("a taken name is a conflict and changes nothing", func(t *testing.T) {
		before := e.categoryRows(a.id)
		for _, name := range []string{"Groceries", "groceries", " TRANSPORT "} {
			rec := e.patchCategory(tok, food.ID.String(), jsonBody(t, map[string]string{"name": name}))
			if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" ||
				!strings.Contains(rec.Body.String(), domain.NameTakenMessage) {
				t.Errorf("rename to %q: %d %s, want 409", name, rec.Code, rec.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused rename changed the categories")
		}
	})

	t.Run("icon set and cleared", func(t *testing.T) {
		// Food is seeded with 🍜 (ADR-0072), so set a different one.
		if got := e.mustPatchCategory(tok, food.ID, `{"icon":" 🍲 "}`); got.Icon != "🍲" {
			t.Errorf("icon = %q", got.Icon)
		}
		if got := e.mustPatchCategory(tok, food.ID, `{"icon":""}`); got.Icon != "" {
			t.Errorf("icon = %q, want cleared", got.Icon)
		}
		rec := e.patchCategory(tok, food.ID.String(), jsonBody(t, map[string]string{"icon": strings.Repeat("i", 33)}))
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
			t.Errorf("33-character icon: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("archive keeps the place; the list still has it, flagged", func(t *testing.T) {
		got := e.mustPatchCategory(tok, groceries.ID, `{"archived":true}`)
		if !got.Archived || got.SortOrder != groceries.SortOrder {
			t.Errorf("archived %+v", got)
		}
		if c := byName(t, e.listCategories(tok), "Groceries"); !c.Archived || c.SortOrder != groceries.SortOrder {
			t.Errorf("listed as %+v", c)
		}
	})

	t.Run("archived set to the value it has changes nothing", func(t *testing.T) {
		before := e.categoryRows(a.id)
		if got := e.mustPatchCategory(tok, groceries.ID, `{"archived":true}`); !got.Archived {
			t.Errorf("got %+v", got)
		}
		if got := e.mustPatchCategory(tok, food.ID, `{"archived":false,"name":"EATING OUT"}`); got.Archived {
			t.Errorf("got %+v", got)
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("a no-op PATCH wrote something")
		}
	})

	t.Run("an archived category can take a taken name", func(t *testing.T) {
		if got := e.mustPatchCategory(tok, groceries.ID, `{"name":"Transport"}`); got.Name != "Transport" || !got.Archived {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("unarchive into a taken name is a conflict and stays archived", func(t *testing.T) {
		before := e.categoryRows(a.id)
		rec := e.patchCategory(tok, groceries.ID.String(), `{"archived":false}`)
		if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" ||
			!strings.Contains(rec.Body.String(), domain.NameTakenMessage) {
			t.Errorf("unarchive: %d %s, want 409", rec.Code, rec.Body.String())
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("the refused unarchive changed the categories")
		}
	})

	t.Run("unarchive goes to the end of the list", func(t *testing.T) {
		pets := e.mustCreateCategory(tok, "Pets")
		got := e.mustPatchCategory(tok, groceries.ID, `{"name":"Groceries","archived":false}`)
		if got.Archived || got.Name != "Groceries" || got.SortOrder != pets.SortOrder+1 {
			t.Errorf("unarchived %+v, want sort_order %d", got, pets.SortOrder+1)
		}
		list := e.listCategories(tok)
		if last := list[len(list)-1]; last.ID != groceries.ID {
			t.Errorf("last = %+v, want Groceries", last)
		}
		assertOrdered(t, list)
	})

	t.Run("kind, other fields, an empty body and bad values are refused", func(t *testing.T) {
		before := e.categoryRows(a.id)
		for _, body := range []string{
			`{"kind":"income"}`, `{"name":"X","kind":"income"}`, `{"sort_order":1}`, `{"owner_id":"` + uuid.NewString() + `"}`,
			`{}`, `{"name":null}`, `{"name":null,"icon":null,"archived":null}`, ``, `{"archived":"yes"}`, `{"name":""}`,
			`{"name":"` + strings.Repeat("x", 51) + `"}`,
		} {
			rec := e.patchCategory(tok, food.ID.String(), body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("PATCH %s: %d %s, want 400", body, rec.Code, rec.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused PATCH changed the categories")
		}
	})

	t.Run("an unknown or malformed id is the same 404", func(t *testing.T) {
		var bodies []string
		for _, id := range []string{uuid.NewString(), "not-a-uuid", "order", "1"} {
			rec := e.patchCategory(tok, id, `{"name":"X"}`)
			if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
				t.Errorf("PATCH %s: %d %s, want 404", id, rec.Code, rec.Body.String())
			}
			bodies = append(bodies, rec.Body.String())
		}
		for _, b := range bodies[1:] {
			if b != bodies[0] {
				t.Errorf("404 bodies differ: %v", bodies)
			}
		}
	})
}

func TestReorderCategories(t *testing.T) {
	e := newAPIEnv(t)

	t.Run("the new order is stored and returned like GET", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		ids := activeIDs(e.listCategories(tok))
		slices.Reverse(ids)
		rec := e.reorder(tok, ids)
		if rec.Code != http.StatusOK {
			t.Fatalf("reorder: %d %s", rec.Code, rec.Body.String())
		}
		var got categoryList
		decodeStrict(t, rec, &got)
		for i, c := range got.Categories {
			if c.ID != ids[i] || c.SortOrder != i+1 {
				t.Errorf("position %d = %s with sort_order %d, want %s with %d", i, c.ID, c.SortOrder, ids[i], i+1)
			}
		}
		if list := e.listCategories(tok); !slices.Equal(list, got.Categories) {
			t.Errorf("GET after reorder differs from the response:\n%v\n%v", list, got.Categories)
		}
	})

	t.Run("archived categories keep their numbers", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		list := e.listCategories(tok)
		shopping, salary := byName(t, list, "Shopping"), byName(t, list, "Salary")
		e.mustPatchCategory(tok, shopping.ID, `{"archived":true}`)
		e.mustPatchCategory(tok, salary.ID, `{"archived":true}`)
		ids := activeIDs(e.listCategories(tok))
		slices.Reverse(ids)
		rec := e.reorder(tok, ids)
		if rec.Code != http.StatusOK {
			t.Fatalf("reorder: %d %s", rec.Code, rec.Body.String())
		}
		var got categoryList
		decodeStrict(t, rec, &got)
		if len(got.Categories) != defaultCategories {
			t.Errorf("response has %d categories, want all %d", len(got.Categories), defaultCategories)
		}
		assertOrdered(t, got.Categories)
		wasArchived := map[uuid.UUID]category{shopping.ID: shopping, salary.ID: salary}
		for _, c := range got.Categories {
			if orig, ok := wasArchived[c.ID]; ok {
				if !c.Archived || c.SortOrder != orig.SortOrder {
					t.Errorf("archived %s: %+v, want sort_order %d", orig.Name, c, orig.SortOrder)
				}
				continue
			}
			if want := slices.Index(ids, c.ID) + 1; c.SortOrder != want {
				t.Errorf("%s: sort_order %d, want %d", c.Name, c.SortOrder, want)
			}
		}
	})

	t.Run("a list that does not match is a conflict and writes nothing", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		list := e.listCategories(tok)
		archived := byName(t, list, "Health")
		e.mustPatchCategory(tok, archived.ID, `{"archived":true}`)
		ids := activeIDs(e.listCategories(tok))
		before := e.categoryRows(a.id)
		for name, bad := range map[string][]uuid.UUID{
			"one missing":                ids[1:],
			"an extra unknown id":        append(slices.Clone(ids), uuid.New()),
			"an unknown id for one":      append(slices.Clone(ids[1:]), uuid.New()),
			"an archived one added":      append(slices.Clone(ids), archived.ID),
			"an archived one for one":    append(slices.Clone(ids[1:]), archived.ID),
			"empty while some is active": {},
		} {
			rec := e.reorder(tok, bad)
			if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" ||
				!strings.Contains(rec.Body.String(), domain.OrderChangedMessage) {
				t.Errorf("%s: %d %s, want 409", name, rec.Code, rec.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused reorder changed the categories")
		}
	})

	t.Run("a repeated id, a non-UUID or a malformed body is 400", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		ids := activeIDs(e.listCategories(tok))
		before := e.categoryRows(a.id)
		rec := e.reorder(tok, append(slices.Clone(ids), ids[0]))
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" ||
			!strings.Contains(rec.Body.String(), domain.RepeatedIDMessage) {
			t.Errorf("repeated id: %d %s, want 400", rec.Code, rec.Body.String())
		}
		idStrings := make([]string, len(ids))
		for i, id := range ids {
			idStrings[i] = `"` + id.String() + `"`
		}
		for name, body := range map[string]string{
			"not a uuid":    `{"ids":[` + strings.Join(idStrings[1:], ",") + `,"not-a-uuid"]}`,
			"a number":      `{"ids":[` + strings.Join(idStrings[1:], ",") + `,5]}`,
			"no ids":        `{}`,
			"null ids":      `{"ids":null}`,
			"not a list":    `{"ids":"` + ids[0].String() + `"}`,
			"no body":       ``,
			"unknown field": `{"ids":[` + strings.Join(idStrings, ",") + `],"owner_id":"` + a.id.String() + `"}`,
		} {
			rec := e.do(http.MethodPut, "/api/categories/order", tok, body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("%s: %d %s, want 400", name, rec.Code, rec.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused reorder changed the categories")
		}
	})

	t.Run("every category archived: the empty list is the whole order", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		e.exec("update categories set archived = true where owner_id = $1", a.id)
		rec := e.reorder(tok, []uuid.UUID{})
		var got categoryList
		decodeStrict(t, rec, &got)
		if rec.Code != http.StatusOK || len(got.Categories) != defaultCategories {
			t.Errorf("reorder []: %d %s", rec.Code, rec.Body.String())
		}
	})
}

// B never sees, changes or reorders A's categories; both can use one name; nothing in a request
// sets the owner (ADR-0014).
func TestCategoriesCrossUser(t *testing.T) {
	e := newAPIEnv(t)
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)
	const privateName = "A's private qxvz"
	aCat := e.mustCreateCategory(aTok, privateName)
	aList := e.listCategories(aTok)

	t.Run("B's list never has A's categories", func(t *testing.T) {
		rec := e.do(http.MethodGet, "/api/categories", bTok, "")
		if strings.Contains(rec.Body.String(), "qxvz") {
			t.Errorf("B's list has A's category: %s", rec.Body.String())
		}
		bList := e.listCategories(bTok)
		if len(bList) != defaultCategories {
			t.Errorf("B has %d categories, want %d", len(bList), defaultCategories)
		}
		for _, c := range bList {
			if slices.ContainsFunc(aList, func(x category) bool { return x.ID == c.ID }) {
				t.Errorf("B's list has A's %s", c.ID)
			}
		}
	})

	t.Run("B's PATCH on A's id is the 404 of an unknown id and changes nothing", func(t *testing.T) {
		aRows := e.categoryRows(a.id)
		unknown := e.patchCategory(bTok, uuid.NewString(), `{"name":"Taken over"}`)
		for _, body := range []string{`{"name":"Taken over"}`, `{"archived":true}`, `{"icon":"x"}`, `{"archived":false}`} {
			rec := e.patchCategory(bTok, aCat.ID.String(), body)
			if rec.Code != http.StatusNotFound || rec.Body.String() != unknown.Body.String() {
				t.Errorf("B's PATCH %s on A's id: %d %s, want %d %s", body, rec.Code, rec.Body.String(), unknown.Code, unknown.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, aRows) {
			t.Error("B's PATCH changed A's categories")
		}
	})

	t.Run("B's reorder with A's id is a conflict and changes neither order", func(t *testing.T) {
		aRows, bRows := e.categoryRows(a.id), e.categoryRows(b.id)
		bIDs := activeIDs(e.listCategories(bTok))
		for name, ids := range map[string][]uuid.UUID{
			"added":       append(slices.Clone(bIDs), aCat.ID),
			"for one":     append(slices.Clone(bIDs[1:]), aCat.ID),
			"only A's":    activeIDs(aList),
			"A's for all": append(activeIDs(aList)[:len(bIDs)-1], bIDs[0]),
		} {
			rec := e.reorder(bTok, ids)
			if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" {
				t.Errorf("%s: %d %s, want 409", name, rec.Code, rec.Body.String())
			}
		}
		if got := e.categoryRows(a.id); !slices.Equal(got, aRows) {
			t.Error("B's reorder changed A's categories")
		}
		if got := e.categoryRows(b.id); !slices.Equal(got, bRows) {
			t.Error("B's refused reorder changed B's categories")
		}
	})

	t.Run("A and B can both have one name", func(t *testing.T) {
		if c := e.mustCreateCategory(bTok, privateName); c.ID == aCat.ID {
			t.Error("B got A's category")
		}
		if n := e.count("select count(*) from categories where lower(name) = lower($1)", privateName); n != 2 {
			t.Errorf("categories named %q = %d, want 2", privateName, n)
		}
	})

	t.Run("nothing in a request sets the owner", func(t *testing.T) {
		rec := e.createCategory(bTok, `{"name":"Owned","kind":"expense","owner_id":"`+a.id.String()+`"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("owner_id in a create: %d %s, want 400", rec.Code, rec.Body.String())
		}
		rec = e.patchCategory(bTok, byName(t, e.listCategories(bTok), "Food").ID.String(), `{"owner_id":"`+a.id.String()+`"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("owner_id in a PATCH: %d %s, want 400", rec.Code, rec.Body.String())
		}
		e.mustCreateCategory(bTok, "Owned")
		if n := e.count("select count(*) from categories where name = 'Owned' and owner_id = $1", b.id); n != 1 {
			t.Errorf("B's new category is not B's (%d)", n)
		}
		if n := e.count("select count(*) from categories where owner_id = $1", a.id); n != len(aList) {
			t.Errorf("A has %d categories, want %d", n, len(aList))
		}
	})
}

// Every /api/categories route answers 401 without a usable session.
func TestCategoriesNeedSession(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	before := e.categoryRows(a.id)
	id := byName(t, e.listCategories(e.freshSession(a.id)), "Food").ID.String()
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/categories", ""},
		{http.MethodPost, "/api/categories", `{"name":"Pets","kind":"expense"}`},
		{http.MethodPatch, "/api/categories/" + id, `{"name":"Pets"}`},
		{http.MethodPut, "/api/categories/order", `{"ids":[]}`},
	}
	for _, r := range routes {
		for _, tok := range []string{"", accountdomain.NewToken().Plain} {
			rec := e.do(r.method, r.path, tok, r.body)
			if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "unauthenticated" {
				t.Errorf("%s %s with token %q: %d %s, want 401", r.method, r.path, tok, rec.Code, rec.Body.String())
			}
		}
	}
	if got := e.categoryRows(a.id); !slices.Equal(got, before) {
		t.Error("an unauthenticated request changed the categories")
	}
}

// Expected unique-key conflicts answer 409 and are logged at info level as "SQL conflict",
// without the conflicting value, never as an ERROR line (ADR-0070).
func TestConflictsLogNoError(t *testing.T) {
	e := newAPIEnv(t)

	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	e.mustCreateCategory(tok, "Conflict qxvz")
	if rec := e.createCategory(tok, `{"name":"CONFLICT QXVZ","kind":"expense"}`); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e.patchCategory(tok, byName(t, e.listCategories(tok), "Food").ID.String(), `{"name":"conflict qxvz"}`); rec.Code != http.StatusConflict {
		t.Fatalf("rename to a taken name: %d %s", rec.Code, rec.Body.String())
	}

	op := e.newAccount(accountOpts{operator: true})
	existing := e.newAccount(accountOpts{})
	if rec := e.invite(e.freshSession(op.id), existing.email); rec.Code != http.StatusConflict {
		t.Fatalf("invite an existing email: %d %s", rec.Code, rec.Body.String())
	}

	logs := e.logs.String()
	if strings.Contains(logs, `"level":"ERROR"`) {
		t.Errorf("an expected conflict was logged at error level:\n%s", logs)
	}
	if n := strings.Count(logs, `"msg":"SQL conflict"`); n != 3 {
		t.Errorf("SQL conflict lines = %d, want 3:\n%s", n, logs)
	}
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, `"msg":"SQL conflict"`) && !strings.Contains(line, `"level":"INFO"`) {
			t.Errorf("conflict line not at info level: %s", line)
		}
	}
	for _, secret := range []string{"qxvz", "QXVZ", existing.email} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain %q:\n%s", secret, logs)
		}
	}
}
