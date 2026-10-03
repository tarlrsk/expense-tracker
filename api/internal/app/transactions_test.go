package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	accountdomain "github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// API-level tests of /api/transactions (PLAN-0002 T8; ADR-0040, ADR-0041, ADR-0042, ADR-0071).

// assertBadRequest fails unless rec is 400 invalid_input whose message contains msg ("" for
// any message).
func assertBadRequest(t *testing.T, what string, rec *httptest.ResponseRecorder, msg string) {
	t.Helper()
	var body httpx.ErrorBody
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusBadRequest || body.Error.Code != "invalid_input" || !strings.Contains(body.Error.Message, msg) {
		t.Errorf("%s: %d %s, want 400 invalid_input with %q", what, rec.Code, rec.Body.String(), msg)
	}
}

func TestCreateTransaction(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food := e.categoryID(tok, "Food")

	t.Run("201 with the whole item; owner_id set, amount text with two decimals", func(t *testing.T) {
		id := newTxID(t)
		rec := e.createTx(tok, jsonBody(t, map[string]any{
			"id": id, "amount": "145", "occurred_on": "2026-01-15", "category_id": food.String(),
			"merchant": "  7-Eleven  ", "note": " lunch\nwith team ", "currency": "THB", "source": "manual",
		}))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
		m := decodeObject(t, rec)
		assertKeys(t, "transaction", m, txItemKeys...)
		if m["amount"] != "145.00" {
			t.Errorf("amount = %#v, want the string \"145.00\"", m["amount"])
		}
		var it txItem
		decodeStrict(t, rec, &it)
		want := txItem{
			ID: uuid.MustParse(id), OwnerID: a.id, Amount: "145.00", Currency: "THB", OccurredOn: "2026-01-15",
			Merchant: "7-Eleven", CategoryID: food, Note: "lunch\nwith team", Source: "manual",
			CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
		}
		if it != want || it.CreatedAt.IsZero() || !it.UpdatedAt.Equal(it.CreatedAt) {
			t.Errorf("created %+v, want %+v", it, want)
		}
		if n := e.count("select count(*) from transactions where id = $1 and owner_id = $2 and amount = 145", id, a.id); n != 1 {
			t.Errorf("stored rows = %d, want 1", n)
		}
	})

	t.Run("optional fields: no merchant, note, currency or source", func(t *testing.T) {
		it := e.mustCreateTx(tok, food, map[string]any{"currency": nil, "source": nil})
		if it.Merchant != "" || it.Note != "" || it.Currency != "THB" || it.Source != "manual" {
			t.Errorf("created %+v", it)
		}
		rec := e.createTx(tok, txBody(t, food, map[string]any{"currency": nil}))
		if rec.Code != http.StatusCreated {
			t.Errorf("null currency: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("the same id again: 200 with the stored item, nothing written, even when fields differ", func(t *testing.T) {
		id := newTxID(t)
		first := e.mustCreateTx(tok, food, map[string]any{"id": id, "amount": "50.5", "merchant": "First"})
		before := e.txRows(a.id)
		transport := e.categoryID(tok, "Transport")
		for _, body := range []string{
			txBody(t, food, map[string]any{"id": id, "amount": "50.5", "merchant": "First"}),
			txBody(t, transport, map[string]any{"id": id, "amount": "999", "merchant": "Other", "occurred_on": "2025-05-05"}),
		} {
			rec := e.createTx(tok, body)
			if rec.Code != http.StatusOK {
				t.Fatalf("again: %d %s, want 200", rec.Code, rec.Body.String())
			}
			var got txItem
			decodeStrict(t, rec, &got)
			if got != first {
				t.Errorf("again returned %+v, want the stored %+v", got, first)
			}
		}
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("a repeated create wrote something")
		}
		if n := e.count("select count(*) from transactions where id = $1", id); n != 1 {
			t.Errorf("rows with the id = %d, want 1", n)
		}
	})

	t.Run("a retry after the category was archived still gets its transaction", func(t *testing.T) {
		pets := e.mustCreateCategory(tok, "Pets "+uuid.NewString()[:8])
		body := txBody(t, pets.ID, nil)
		if rec := e.createTx(tok, body); rec.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
		e.mustPatchCategory(tok, pets.ID, `{"archived":true}`)
		if rec := e.createTx(tok, body); rec.Code != http.StatusOK {
			t.Errorf("retry: %d %s, want 200", rec.Code, rec.Body.String())
		}
	})

	t.Run("two creates with one id at the same moment: one row, one 201, one 200", func(t *testing.T) {
		for range 5 {
			id := newTxID(t)
			body := txBody(t, food, map[string]any{"id": id})
			codes := concurrently(
				func() int { return e.createTx(tok, body).Code },
				func() int { return e.createTx(tok, body).Code },
			)
			if !slices.Equal(codes, []int{http.StatusOK, http.StatusCreated}) {
				t.Errorf("statuses = %v, want 200 and 201", codes)
			}
			if n := e.count("select count(*) from transactions where id = $1", id); n != 1 {
				t.Errorf("rows with the id = %d, want 1", n)
			}
		}
	})

	t.Run("id: required, a UUID version 7 in the usual form", func(t *testing.T) {
		before := e.txRows(a.id)
		v7 := newTxID(t)
		for name, id := range map[string]any{
			"missing":        nil,
			"empty":          "",
			"not a uuid":     "not-a-uuid",
			"version 4":      uuid.NewString(),
			"nil uuid":       uuid.Nil.String(),
			"max uuid":       "ffffffff-ffff-ffff-ffff-ffffffffffff",
			"without dashes": strings.ReplaceAll(v7, "-", ""),
			"in braces":      "{" + v7 + "}",
			"urn":            "urn:uuid:" + v7,
		} {
			rec := e.createTx(tok, txBody(t, food, map[string]any{"id": id}))
			assertBadRequest(t, name, rec, domain.IDRuleMessage)
		}
		rec := e.createTx(tok, txBody(t, food, map[string]any{"id": 7}))
		assertBadRequest(t, "a number", rec, "")
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused create wrote something")
		}
		upper := strings.ToUpper(newTxID(t))
		if it := e.mustCreateTx(tok, food, map[string]any{"id": upper}); it.ID.String() != strings.ToLower(upper) {
			t.Errorf("upper-case id stored as %s", it.ID)
		}
	})

	t.Run("amount rules", func(t *testing.T) {
		for _, c := range []struct {
			in   any
			want string // "" means refused
		}{
			{"0", ""}, {"0.00", ""}, {"-1", ""}, {"1.234", ""}, {"1e3", ""}, {" 1", ""}, {"1 ", ""}, {"1.", ""},
			{".5", ""}, {"", ""}, {"+1", ""}, {"1,000", ""}, {"๑๒", ""}, {"0x10", ""}, {"NaN", ""}, {"Infinity", ""},
			{145, ""}, {145.5, ""}, {nil, ""}, {true, ""},
			{"10000000000.00", ""}, {"10000000000", ""}, {"99999999999", ""}, {"9999999999.999", ""},
			{"9999999999.99", "9999999999.99"}, {"0.01", "0.01"}, {"145", "145.00"}, {"145.5", "145.50"},
			{"0145.50", "145.50"}, {"00000000000001", "1.00"}, {"1.10", "1.10"},
		} {
			rec := e.createTx(tok, txBody(t, food, map[string]any{"amount": c.in}))
			if c.want == "" {
				assertBadRequest(t, "amount "+jsonBody(t, c.in), rec, "")
				continue
			}
			var it txItem
			decodeStrict(t, rec, &it)
			if rec.Code != http.StatusCreated || it.Amount != c.want {
				t.Errorf("amount %q: %d %s, want %q", c.in, rec.Code, rec.Body.String(), c.want)
			}
			if n := e.count("select count(*) from transactions where id = $1 and amount::text = $2", it.ID, c.want); n != 1 {
				t.Errorf("amount %q is not stored as %s", c.in, c.want)
			}
		}
		rec := e.createTx(tok, txBody(t, food, map[string]any{"amount": "1.234"}))
		assertBadRequest(t, "three decimals", rec, domain.AmountRuleMessage)
	})

	t.Run("currency: only THB", func(t *testing.T) {
		for _, cur := range []string{"USD", "thb", "", " THB", "EUR"} {
			rec := e.createTx(tok, txBody(t, food, map[string]any{"currency": cur}))
			assertBadRequest(t, "currency "+cur, rec, domain.CurrencyMessage)
		}
	})

	t.Run("source: only manual", func(t *testing.T) {
		for _, src := range []string{"text", "scan", "csv", "", "Manual", "other"} {
			rec := e.createTx(tok, txBody(t, food, map[string]any{"source": src}))
			assertBadRequest(t, "source "+src, rec, domain.SourceMessage)
		}
	})

	t.Run("merchant and note: trimmed, limited, no control characters", func(t *testing.T) {
		long := func(n int) string { return strings.Repeat("ก", n) }
		ok := []struct{ merchant, note, wantMerchant, wantNote string }{
			{" \t" + long(100) + " ", long(500) + "\n ", long(100), long(500)},
			{"   ", "\n\n", "", ""},
			{"Café", "line 1\nline 2", "Café", "line 1\nline 2"},
		}
		for _, c := range ok {
			it := e.mustCreateTx(tok, food, map[string]any{"merchant": c.merchant, "note": c.note})
			if it.Merchant != c.wantMerchant || it.Note != c.wantNote {
				t.Errorf("merchant %q note %q stored as %q %q", c.merchant, c.note, it.Merchant, it.Note)
			}
		}
		for _, m := range []string{long(101), "a\nb", "a\tb", "a\x00b", "a\x1bb", "a\u0085b"} {
			rec := e.createTx(tok, txBody(t, food, map[string]any{"merchant": m}))
			assertBadRequest(t, "merchant", rec, domain.MerchantRuleMessage)
		}
		for _, n := range []string{long(501), "a\tb", "a\rb", "a\x00b", "a\x7fb"} {
			rec := e.createTx(tok, txBody(t, food, map[string]any{"note": n}))
			assertBadRequest(t, "note", rec, domain.NoteRuleMessage)
		}
	})

	t.Run("category: an archived, unknown, malformed or missing one is the same 400", func(t *testing.T) {
		groceries := e.categoryID(tok, "Groceries")
		e.mustPatchCategory(tok, groceries, `{"archived":true}`)
		before := e.txRows(a.id)
		var bodies []string
		for name, cat := range map[string]any{
			"archived": groceries.String(), "unknown": uuid.NewString(), "malformed": "food", "missing": nil, "nil": uuid.Nil.String(),
		} {
			rec := e.createTx(tok, txBody(t, food, map[string]any{"category_id": cat}))
			assertBadRequest(t, name, rec, domain.CategoryMessage)
			bodies = append(bodies, rec.Body.String())
		}
		for _, b := range bodies[1:] {
			if b != bodies[0] {
				t.Errorf("category 400 bodies differ: %v", bodies)
			}
		}
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused create wrote something")
		}
	})

	t.Run("with every category archived, adding is refused until one is active again", func(t *testing.T) {
		e := e.with(t)
		b := e.newAccount(accountOpts{})
		bTok := e.freshSession(b.id)
		bFood := e.categoryID(bTok, "Food")
		e.exec("update categories set archived = true where owner_id = $1", b.id)
		rec := e.createTx(bTok, txBody(t, bFood, nil))
		assertBadRequest(t, "no active category", rec, domain.CategoryMessage)
		e.mustPatchCategory(bTok, bFood, `{"archived":false}`)
		e.mustCreateTx(bTok, bFood, nil)
	})

	t.Run("malformed bodies and unknown fields are refused", func(t *testing.T) {
		before := e.txRows(a.id)
		for _, body := range []string{
			``, `{`, `[]`, `null`,
			txBody(t, food, map[string]any{"owner_id": a.id.String()}),
			txBody(t, food, map[string]any{"raw_input": "lunch 145"}),
			txBody(t, food, map[string]any{"created_at": "2026-01-01T00:00:00Z"}),
			txBody(t, food, map[string]any{"updated_at": "2026-01-01T00:00:00Z"}),
			txBody(t, food, nil) + ` {}`,
		} {
			rec := e.createTx(tok, body)
			assertBadRequest(t, body, rec, "")
		}
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused create wrote something")
		}
	})
}

// The date rule uses the app time zone: with the clock at 17:30 UTC, Bangkok (UTC+7) is already
// on the next day, so its "one year after today" is a day later than UTC's (ADR-0042, ADR-0071).
func TestTransactionDates(t *testing.T) {
	utcNow := time.Now().UTC()
	clock := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 17, 30, 0, 0, time.UTC)
	day := func(t time.Time) string { return t.Format("2006-01-02") }
	bangkokToday := time.Date(clock.Year(), clock.Month(), clock.Day()+1, 0, 0, 0, 0, time.UTC)
	bangkokMax, utcMax := day(bangkokToday.AddDate(1, 0, 0)), day(clock.AddDate(1, 0, 0))
	afterBangkokMax := day(bangkokToday.AddDate(1, 0, 1))
	if bangkokMax == utcMax {
		t.Fatalf("the clock does not separate the zones: %s", bangkokMax)
	}

	e := newAPIEnv(t, withClock(clock))
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food := e.categoryID(tok, "Food")

	for _, c := range []struct {
		date string
		ok   bool
	}{
		{"2000-01-01", true}, {"1999-12-31", false}, {"0001-01-01", false},
		{bangkokMax, true}, {utcMax, true}, {afterBangkokMax, false}, {"9999-12-31", false},
		{"2026-02-28", true}, {"2024-02-29", true}, {"2026-02-29", false}, {"2026-02-30", false}, {"2026-04-31", false},
		{"2026-13-01", false}, {"2026-00-10", false}, {"2026-01-00", false},
		{"2026-2-3", false}, {"2026-02-3", false}, {"26-02-03", false}, {"20260203", false}, {"2026/02/03", false},
		{"03-02-2026", false}, {"2026-02-03T00:00:00Z", false}, {"2026-02-03 ", false}, {" 2026-02-03", false},
		{"+2026-02-03", false}, {"", false}, {"today", false},
	} {
		rec := e.createTx(tok, txBody(t, food, map[string]any{"occurred_on": c.date}))
		if !c.ok {
			assertBadRequest(t, "date "+c.date, rec, domain.DateRuleMessage)
			continue
		}
		var it txItem
		decodeStrict(t, rec, &it)
		if rec.Code != http.StatusCreated || it.OccurredOn != c.date {
			t.Errorf("date %q: %d %s", c.date, rec.Code, rec.Body.String())
		}
	}
	rec := e.createTx(tok, txBody(t, food, map[string]any{"occurred_on": 20260203}))
	assertBadRequest(t, "a number", rec, "")

	t.Run("PATCH uses the same rule", func(t *testing.T) {
		it := e.mustCreateTx(tok, food, nil)
		if got := e.mustPatchTx(tok, it.ID, `{"occurred_on":"`+bangkokMax+`"}`); got.OccurredOn != bangkokMax {
			t.Errorf("occurred_on = %s", got.OccurredOn)
		}
		for _, d := range []string{afterBangkokMax, "1999-12-31", "2026-02-30", ""} {
			rec := e.patchTx(tok, it.ID.String(), `{"occurred_on":"`+d+`"}`)
			assertBadRequest(t, "PATCH date "+d, rec, domain.DateRuleMessage)
		}
	})

	t.Run("in UTC the same clock allows one day less", func(t *testing.T) {
		e := newAPIEnv(t, withClock(clock), withZone(time.UTC))
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		food := e.categoryID(tok, "Food")
		rec := e.createTx(tok, txBody(t, food, map[string]any{"occurred_on": bangkokMax}))
		assertBadRequest(t, "UTC "+bangkokMax, rec, domain.DateRuleMessage)
		e.mustCreateTx(tok, food, map[string]any{"occurred_on": utcMax})
	})
}

func TestUpdateTransaction(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")
	orig := e.mustCreateTx(tok, food, map[string]any{"amount": "145", "merchant": "Shop", "note": "n", "occurred_on": "2026-01-15"})

	t.Run("each field; the others, the id, owner, source and created_at stay", func(t *testing.T) {
		prev := orig
		for _, c := range []struct {
			body  string
			check func(it txItem) bool
		}{
			{`{"amount":"99.9"}`, func(it txItem) bool { return it.Amount == "99.90" }},
			{`{"occurred_on":"2025-12-31"}`, func(it txItem) bool { return it.OccurredOn == "2025-12-31" }},
			{`{"category_id":"` + transport.String() + `"}`, func(it txItem) bool { return it.CategoryID == transport }},
			{`{"merchant":"  BTS  "}`, func(it txItem) bool { return it.Merchant == "BTS" }},
			{`{"note":" to work\nand back "}`, func(it txItem) bool { return it.Note == "to work\nand back" }},
			{`{"merchant":""}`, func(it txItem) bool { return it.Merchant == "" }},
			{`{"note":""}`, func(it txItem) bool { return it.Note == "" }},
		} {
			it := e.mustPatchTx(tok, orig.ID, c.body)
			if !c.check(it) {
				t.Errorf("PATCH %s gave %+v", c.body, it)
			}
			if it.ID != orig.ID || it.OwnerID != a.id || it.Source != "manual" || it.Currency != "THB" ||
				!it.CreatedAt.Equal(orig.CreatedAt) || !it.UpdatedAt.After(prev.UpdatedAt) {
				t.Errorf("PATCH %s: %+v after %+v", c.body, it, prev)
			}
			prev = it
		}
		it := e.mustPatchTx(tok, orig.ID, `{"amount":"1","occurred_on":"2026-01-01","category_id":"`+food.String()+
			`","merchant":"M","note":"N","currency":"THB"}`)
		want := txItem{
			ID: orig.ID, OwnerID: a.id, Amount: "1.00", Currency: "THB", OccurredOn: "2026-01-01", Merchant: "M",
			CategoryID: food, Note: "N", Source: "manual", CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
		}
		if it != want {
			t.Errorf("all fields: %+v, want %+v", it, want)
		}
		listed := e.listTxs(tok, nil).Transactions
		if i := slices.IndexFunc(listed, func(x txItem) bool { return x.ID == it.ID }); i < 0 || listed[i] != it {
			t.Errorf("the list does not show the change: %+v", listed)
		}
	})

	t.Run("a PATCH that changes nothing writes nothing", func(t *testing.T) {
		cur := e.mustPatchTx(tok, orig.ID, `{"amount":"145","merchant":"Shop","note":"n","occurred_on":"2026-01-15","category_id":"`+food.String()+`"}`)
		before := e.txRows(a.id)
		for _, body := range []string{
			`{"amount":"145.00"}`, `{"amount":"145.0"}`, `{"merchant":" Shop "}`, `{"note":"n\n"}`, `{"currency":"THB"}`,
			`{"occurred_on":"2026-01-15","category_id":"` + food.String() + `"}`, `{"amount":"145","note":null}`,
		} {
			if got := e.mustPatchTx(tok, orig.ID, body); got != cur {
				t.Errorf("PATCH %s returned %+v, want %+v", body, got, cur)
			}
		}
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("a no-op PATCH wrote something (updated_at moved?)")
		}
	})

	t.Run("an empty body, unknown or fixed fields and bad values are refused", func(t *testing.T) {
		before := e.txRows(a.id)
		for _, body := range []string{
			`{}`, `{"amount":null}`, `{"amount":null,"merchant":null,"note":null,"occurred_on":null,"category_id":null,"currency":null}`,
			``, `[]`, `{"source":"manual"}`, `{"source":"text"}`, `{"owner_id":"` + uuid.NewString() + `"}`,
			`{"id":"` + newTxID(t) + `"}`, `{"raw_input":"x"}`, `{"created_at":"2026-01-01T00:00:00Z"}`,
			`{"amount":"0"}`, `{"amount":145}`, `{"amount":"1.234"}`, `{"currency":"USD"}`, `{"currency":""}`,
			`{"category_id":"x"}`, `{"category_id":"` + uuid.NewString() + `"}`, `{"merchant":"a\tb"}`,
			`{"note":"` + strings.Repeat("x", 501) + `"}`, `{"occurred_on":"2026-02-30"}`, `{"amount":"2"} {}`,
		} {
			rec := e.patchTx(tok, orig.ID.String(), body)
			assertBadRequest(t, "PATCH "+body, rec, "")
		}
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("a refused PATCH wrote something")
		}
	})

	t.Run("an archived category stays usable for the transaction that has it, not as a new one", func(t *testing.T) {
		pets := e.mustCreateCategory(tok, "Pets")
		toys := e.mustCreateCategory(tok, "Toys")
		it := e.mustCreateTx(tok, pets.ID, map[string]any{"amount": "10"})
		e.mustPatchCategory(tok, pets.ID, `{"archived":true}`)
		e.mustPatchCategory(tok, toys.ID, `{"archived":true}`)

		if got := e.mustPatchTx(tok, it.ID, `{"amount":"20"}`); got.Amount != "20.00" || got.CategoryID != pets.ID {
			t.Errorf("amount change on an archived category: %+v", got)
		}
		before := e.txRows(a.id)
		if got := e.mustPatchTx(tok, it.ID, `{"category_id":"`+pets.ID.String()+`"}`); got.CategoryID != pets.ID {
			t.Errorf("keeping the archived category: %+v", got)
		}
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("sending the category it already has wrote something")
		}
		if got := e.mustPatchTx(tok, it.ID, `{"category_id":"`+pets.ID.String()+`","amount":"30"}`); got.Amount != "30.00" {
			t.Errorf("keeping the category with an amount change: %+v", got)
		}
		before = e.txRows(a.id)
		rec := e.patchTx(tok, it.ID.String(), `{"category_id":"`+toys.ID.String()+`"}`)
		assertBadRequest(t, "to another archived category", rec, domain.CategoryMessage)
		if got := e.txRows(a.id); !slices.Equal(got, before) {
			t.Error("the refused category change wrote something")
		}
		if got := e.mustPatchTx(tok, it.ID, `{"category_id":"`+transport.String()+`"}`); got.CategoryID != transport {
			t.Errorf("to an active category: %+v", got)
		}
		rec = e.patchTx(tok, it.ID.String(), `{"category_id":"`+pets.ID.String()+`"}`)
		assertBadRequest(t, "back to the archived category", rec, domain.CategoryMessage)
	})

	t.Run("an unknown or malformed id is the same 404", func(t *testing.T) {
		var bodies []string
		for _, id := range []string{newTxID(t), uuid.NewString(), "not-a-uuid", "1"} {
			rec := e.patchTx(tok, id, `{"amount":"1"}`)
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

func TestDeleteTransaction(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food := e.categoryID(tok, "Food")
	keep := e.mustCreateTx(tok, food, nil)
	it := e.mustCreateTx(tok, food, nil)

	rec := e.deleteTx(tok, it.ID.String())
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %q, want 204", rec.Code, rec.Body.String())
	}
	if n := e.count("select count(*) from transactions where id = $1", it.ID); n != 0 {
		t.Errorf("the row is still there")
	}
	if n := e.count("select count(*) from transactions where id = $1", keep.ID); n != 1 {
		t.Errorf("the other row is gone")
	}
	var bodies []string
	for _, id := range []string{it.ID.String(), newTxID(t), "not-a-uuid"} {
		rec := e.deleteTx(tok, id)
		if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
			t.Errorf("delete %s: %d %s, want 404", id, rec.Code, rec.Body.String())
		}
		bodies = append(bodies, rec.Body.String())
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Errorf("404 bodies differ: %v", bodies)
		}
	}
	if rec := e.patchTx(tok, it.ID.String(), `{"amount":"1"}`); rec.Code != http.StatusNotFound {
		t.Errorf("PATCH after delete: %d", rec.Code)
	}
	if rec := e.createTx(tok, txBody(t, food, map[string]any{"id": it.ID.String()})); rec.Code != http.StatusCreated {
		t.Errorf("create with the deleted id: %d %s", rec.Code, rec.Body.String())
	}
}

func TestListTransactions(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")
	dates := []string{"2026-01-31", "2026-02-01", "2026-02-14", "2026-02-14", "2026-02-28", "2026-03-01", "2025-12-31"}
	byDate := map[string][]uuid.UUID{}
	for i, d := range dates {
		cat := food
		if i%2 == 1 {
			cat = transport
		}
		it := e.mustCreateTx(tok, cat, map[string]any{"occurred_on": d})
		byDate[d] = append(byDate[d], it.ID)
	}
	e.seedTxs(a.id, food, 1, "text", "'2026-02-10'")
	e.seedTxs(a.id, food, 1, "scan", "'2026-02-11'")
	e.seedTxs(a.id, transport, 1, "csv", "'2026-02-12'")
	total := len(dates) + 3

	all := e.listTxs(tok, nil)
	t.Run("everything, newest first, then by id descending; shape", func(t *testing.T) {
		rec := e.getTxs(tok, nil)
		body := decodeObject(t, rec)
		assertKeys(t, "list", body, "transactions", "next_cursor")
		if body["next_cursor"] != nil {
			t.Errorf("next_cursor = %v, want null", body["next_cursor"])
		}
		for _, it := range body["transactions"].([]any) {
			assertKeys(t, "transaction", it.(map[string]any), txItemKeys...)
		}
		if len(all.Transactions) != total {
			t.Fatalf("transactions = %d, want %d", len(all.Transactions), total)
		}
		assertTxOrder(t, all.Transactions)
	})

	dateSet := func(items []txItem) []string {
		var out []string
		for _, it := range items {
			out = append(out, it.OccurredOn)
		}
		return out
	}
	for _, c := range []struct {
		name  string
		q     url.Values
		dates []string
	}{
		{"month", query("month", "2026-02"), []string{"2026-02-28", "2026-02-14", "2026-02-14", "2026-02-12", "2026-02-11", "2026-02-10", "2026-02-01"}},
		{"another month", query("month", "2025-12"), []string{"2025-12-31"}},
		{"an empty month", query("month", "2024-02"), nil},
		{"from and to, inclusive", query("from", "2026-02-01", "to", "2026-02-14"), []string{"2026-02-14", "2026-02-14", "2026-02-12", "2026-02-11", "2026-02-10", "2026-02-01"}},
		{"from alone", query("from", "2026-02-28"), []string{"2026-03-01", "2026-02-28"}},
		{"to alone", query("to", "2026-01-31"), []string{"2026-01-31", "2025-12-31"}},
		{"one day", query("from", "2026-02-14", "to", "2026-02-14"), []string{"2026-02-14", "2026-02-14"}},
		{"a week", query("from", "2026-02-09", "to", "2026-02-15"), []string{"2026-02-14", "2026-02-14", "2026-02-12", "2026-02-11", "2026-02-10"}},
		{"source text", query("source", "text"), []string{"2026-02-10"}},
		{"source csv", query("source", "csv"), []string{"2026-02-12"}},
		{"source manual in a month", query("source", "manual", "month", "2026-02"), []string{"2026-02-28", "2026-02-14", "2026-02-14", "2026-02-01"}},
		{"category", query("category", transport.String()), []string{"2026-03-01", "2026-02-14", "2026-02-12", "2026-02-01"}},
		{"category and period", query("category", food.String(), "from", "2026-02-11"), []string{"2026-02-28", "2026-02-14", "2026-02-11"}},
		{"a category that is not the caller's", query("category", uuid.NewString()), nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := e.listTxs(tok, c.q)
			if d := dateSet(got.Transactions); !slices.Equal(d, c.dates) {
				t.Errorf("dates = %v, want %v", d, c.dates)
			}
			assertTxOrder(t, got.Transactions)
			if got.NextCursor != nil {
				t.Errorf("next_cursor = %q, want null", *got.NextCursor)
			}
		})
	}

	t.Run("an archived category can still be listed", func(t *testing.T) {
		e.mustPatchCategory(tok, transport, `{"archived":true}`)
		defer e.mustPatchCategory(tok, transport, `{"archived":false}`)
		if got := e.listTxs(tok, query("category", transport.String())); len(got.Transactions) != 4 {
			t.Errorf("transactions = %d, want 4", len(got.Transactions))
		}
	})

	t.Run("limit", func(t *testing.T) {
		for _, n := range []int{1, 3, total, 200} {
			got := e.listTxs(tok, query("limit", strconv.Itoa(n)))
			want := min(n, total)
			if len(got.Transactions) != want || !slices.Equal(txIDs(got.Transactions), txIDs(all.Transactions[:want])) {
				t.Errorf("limit %d: %d transactions", n, len(got.Transactions))
			}
			if (got.NextCursor != nil) != (n < total) {
				t.Errorf("limit %d: next_cursor %v", n, got.NextCursor)
			}
		}
	})

	t.Run("malformed, combined or unknown parameters are 400", func(t *testing.T) {
		for _, raw := range []string{
			"month=2026-2", "month=2026-13", "month=2026-02-01", "month=", "month", "month=02-2026",
			"month=2026-02&from=2026-02-01", "month=2026-02&to=2026-02-28",
			"from=2026-02-30", "from=2026-2-1", "to=", "from=2026-02-15&to=2026-02-14",
			"category=food", "category=", "source=Manual", "source=", "source=other",
			"limit=0", "limit=201", "limit=-1", "limit=abc", "limit=", "limit=1.5", "limit=+5", "limit=1000",
			"cursor=", "cursor=abc", "cursor=%25%25", "cursor=" + url.QueryEscape("2026-02-01_"+uuid.NewString()),
			"cursor=MjAyNi0wMi0zMF8wMTkyYmQ4Mi0wMDAwLTcwMDAtODAwMC0wMDAwMDAwMDAwMDA",
			"page=2", "offset=10", "sort=asc", "Month=2026-02", "owner_id=" + a.id.String(),
			"month=2026-02&month=2026-03", "limit=1&limit=2", "source=manual&source=text",
			"month=%zz", "a=1;b=2",
		} {
			rec := e.do(http.MethodGet, "/api/transactions?"+raw, tok, "")
			assertBadRequest(t, raw, rec, "")
		}
	})
}

// assertTxOrder fails unless items are ordered by occurred_on descending, then id descending.
func assertTxOrder(t *testing.T, items []txItem) {
	t.Helper()
	sorted := slices.IsSortedFunc(items, func(a, b txItem) int {
		if c := strings.Compare(b.OccurredOn, a.OccurredOn); c != 0 {
			return c
		}
		return strings.Compare(b.ID.String(), a.ID.String())
	})
	if !sorted {
		t.Errorf("not ordered by occurred_on, then id, descending: %v", items)
	}
}

// Keyset paging over 120 rows, many on one date: every row once, in order (ADR-0071).
func TestListTransactionsPaging(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")
	// 70 rows on 2026-03-10, 50 spread over the days before it; even g in Food, odd g in Transport.
	e.seedTxs(a.id, food, 60, "manual", "case when g <= 35 then '2026-03-10'::date else '2026-03-10'::date - (g - 35) end")
	e.seedTxs(a.id, transport, 60, "manual", "case when g <= 35 then '2026-03-10'::date else '2026-03-10'::date - (g % 20) end")
	const total = 120

	whole := e.listTxs(tok, query("limit", "200"))
	if len(whole.Transactions) != total || whole.NextCursor != nil {
		t.Fatalf("one page of 200: %d rows, next %v", len(whole.Transactions), whole.NextCursor)
	}
	assertTxOrder(t, whole.Transactions)

	t.Run("limit 50 gives 50, 50, 20 with no row repeated or missed", func(t *testing.T) {
		got, sizes := e.listAllTxs(tok, nil, 50)
		if !slices.Equal(sizes, []int{50, 50, 20}) {
			t.Errorf("page sizes = %v, want [50 50 20]", sizes)
		}
		if !slices.Equal(txIDs(got), txIDs(whole.Transactions)) {
			t.Error("paged rows differ from the single page")
		}
	})

	t.Run("the default page size is 50", func(t *testing.T) {
		got := e.listTxs(tok, nil)
		if len(got.Transactions) != 50 || got.NextCursor == nil {
			t.Errorf("first page: %d rows, next %v", len(got.Transactions), got.NextCursor)
		}
	})

	t.Run("pages of 7 and of 1", func(t *testing.T) {
		for _, limit := range []int{7, 1} {
			got, sizes := e.listAllTxs(tok, nil, limit)
			if !slices.Equal(txIDs(got), txIDs(whole.Transactions)) {
				t.Errorf("limit %d: paged rows differ from the single page", limit)
			}
			if sizes[len(sizes)-1] == 0 {
				t.Errorf("limit %d: the last page is empty", limit)
			}
		}
	})

	t.Run("filters combined with paging", func(t *testing.T) {
		for _, q := range []url.Values{
			query("category", transport.String()),
			query("from", "2026-03-01", "to", "2026-03-10"),
			query("month", "2026-02", "category", food.String()),
			query("source", "manual", "to", "2026-03-09"),
		} {
			want := e.listTxs(tok, mergeQuery(q, query("limit", "200")))
			got, _ := e.listAllTxs(tok, q, 9)
			if len(want.Transactions) == 0 || !slices.Equal(txIDs(got), txIDs(want.Transactions)) {
				t.Errorf("%s: %d paged rows, want %d", q.Encode(), len(got), len(want.Transactions))
			}
		}
	})

	t.Run("rows added before the cursor do not shift the pages", func(t *testing.T) {
		first := e.listTxs(tok, query("limit", "50"))
		e.seedTxs(a.id, food, 5, "manual", "'2026-04-01'")
		rest, _ := e.listAllTxs(tok, query("cursor", *first.NextCursor), 50)
		if !slices.Equal(txIDs(append(first.Transactions, rest...)), txIDs(whole.Transactions)) {
			t.Error("the rows after the cursor changed")
		}
	})
}

func mergeQuery(qs ...url.Values) url.Values {
	out := url.Values{}
	for _, q := range qs {
		for k, vs := range q {
			out[k] = append(out[k], vs...)
		}
	}
	return out
}

// B never sees, changes, deletes or takes over A's transactions, nor uses A's categories
// (ADR-0014, ADR-0040).
func TestTransactionsCrossUser(t *testing.T) {
	e := newAPIEnv(t)
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)
	aFood, bFood := e.categoryID(aTok, "Food"), e.categoryID(bTok, "Food")
	var aTx txItem
	for i := range 6 {
		aTx = e.mustCreateTx(aTok, aFood, map[string]any{"merchant": "A's private qxvz", "occurred_on": "2026-02-1" + strconv.Itoa(i)})
	}
	e.seedTxs(a.id, aFood, 2, "scan", "'2026-02-05'")
	bTx := e.mustCreateTx(bTok, bFood, map[string]any{"occurred_on": "2026-02-12"})
	aRows := e.txRows(a.id)

	t.Run("B's list never has A's rows, with every filter", func(t *testing.T) {
		for _, q := range []url.Values{
			nil, query("month", "2026-02"), query("from", "2026-01-01", "to", "2026-12-31"), query("from", "2026-02-10"),
			query("to", "2026-02-28"), query("category", aFood.String()), query("source", "manual"), query("source", "scan"),
			query("limit", "200"), query("limit", "1"),
		} {
			rec := e.getTxs(bTok, q)
			if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "qxvz") || strings.Contains(rec.Body.String(), a.id.String()) {
				t.Errorf("B's list ?%s: %d %s", q.Encode(), rec.Code, rec.Body.String())
			}
			for _, it := range e.listTxs(bTok, q).Transactions {
				if it.OwnerID != b.id || it.ID != bTx.ID {
					t.Errorf("B's list ?%s has %+v", q.Encode(), it)
				}
			}
		}
	})

	t.Run("A's cursor used by B positions only inside B's rows", func(t *testing.T) {
		page := e.listTxs(aTok, query("limit", "2"))
		if page.NextCursor == nil {
			t.Fatal("A has no second page")
		}
		got := e.listTxs(bTok, query("cursor", *page.NextCursor))
		for _, it := range got.Transactions {
			if it.OwnerID != b.id {
				t.Errorf("B got %+v with A's cursor", it)
			}
		}
		if len(got.Transactions) != 1 {
			t.Errorf("B got %d rows after A's cursor (2026-02-14), want B's one of 2026-02-12", len(got.Transactions))
		}
	})

	t.Run("B's PATCH and DELETE on A's id are the 404 of an unknown id and change nothing", func(t *testing.T) {
		unknownPatch := e.patchTx(bTok, newTxID(t), `{"amount":"1"}`)
		unknownDelete := e.deleteTx(bTok, newTxID(t))
		for _, body := range []string{`{"amount":"1"}`, `{"merchant":"taken"}`, `{"category_id":"` + bFood.String() + `"}`, `{"currency":"THB"}`} {
			rec := e.patchTx(bTok, aTx.ID.String(), body)
			if rec.Code != http.StatusNotFound || rec.Body.String() != unknownPatch.Body.String() {
				t.Errorf("B's PATCH %s: %d %s, want %s", body, rec.Code, rec.Body.String(), unknownPatch.Body.String())
			}
		}
		rec := e.deleteTx(bTok, aTx.ID.String())
		if rec.Code != http.StatusNotFound || rec.Body.String() != unknownDelete.Body.String() {
			t.Errorf("B's DELETE: %d %s, want %s", rec.Code, rec.Body.String(), unknownDelete.Body.String())
		}
		if got := e.txRows(a.id); !slices.Equal(got, aRows) {
			t.Error("B changed A's transactions")
		}
	})

	t.Run("B creating with A's id is the 400 of a malformed id; A's row stays, B gets none", func(t *testing.T) {
		bRows := e.txRows(b.id)
		malformed := e.createTx(bTok, txBody(t, bFood, map[string]any{"id": "not-a-uuid"}))
		for _, fields := range []map[string]any{
			{"id": aTx.ID.String()},
			{"id": aTx.ID.String(), "amount": aTx.Amount, "occurred_on": aTx.OccurredOn, "merchant": aTx.Merchant},
		} {
			rec := e.createTx(bTok, txBody(t, bFood, fields))
			if rec.Code != http.StatusBadRequest || rec.Body.String() != malformed.Body.String() {
				t.Errorf("B's create with A's id: %d %s, want %s", rec.Code, rec.Body.String(), malformed.Body.String())
			}
		}
		codes := concurrently(
			func() int { return e.createTx(bTok, txBody(t, bFood, map[string]any{"id": aTx.ID.String()})).Code },
			func() int { return e.createTx(bTok, txBody(t, bFood, map[string]any{"id": aTx.ID.String()})).Code },
		)
		if !slices.Equal(codes, []int{http.StatusBadRequest, http.StatusBadRequest}) {
			t.Errorf("concurrent: %v", codes)
		}
		if got := e.txRows(a.id); !slices.Equal(got, aRows) {
			t.Error("B's create changed A's transactions")
		}
		if got := e.txRows(b.id); !slices.Equal(got, bRows) {
			t.Error("B's refused create wrote a row for B")
		}
	})

	t.Run("B cannot use A's category", func(t *testing.T) {
		unknown := e.createTx(bTok, txBody(t, uuid.New(), nil))
		rec := e.createTx(bTok, txBody(t, aFood, nil))
		if rec.Code != http.StatusBadRequest || rec.Body.String() != unknown.Body.String() {
			t.Errorf("create with A's category: %d %s, want %s", rec.Code, rec.Body.String(), unknown.Body.String())
		}
		rec = e.patchTx(bTok, bTx.ID.String(), `{"category_id":"`+aFood.String()+`"}`)
		if rec.Code != http.StatusBadRequest || rec.Body.String() != unknown.Body.String() {
			t.Errorf("PATCH to A's category: %d %s, want %s", rec.Code, rec.Body.String(), unknown.Body.String())
		}
		if n := e.count("select count(*) from transactions where owner_id = $1 and category_id = $2", b.id, aFood); n != 0 {
			t.Errorf("B has %d transactions in A's category", n)
		}
	})

	t.Run("nothing in a request sets the owner", func(t *testing.T) {
		rec := e.createTx(bTok, txBody(t, bFood, map[string]any{"owner_id": a.id.String()}))
		assertBadRequest(t, "owner_id in a create", rec, "")
		rec = e.patchTx(bTok, bTx.ID.String(), `{"owner_id":"`+a.id.String()+`"}`)
		assertBadRequest(t, "owner_id in a PATCH", rec, "")
		rec = e.do(http.MethodGet, "/api/transactions?owner_id="+a.id.String(), bTok, "")
		assertBadRequest(t, "owner_id in the list", rec, "")
		if got := e.txRows(a.id); !slices.Equal(got, aRows) {
			t.Error("A's transactions changed")
		}
	})
}

// Every /api/transactions route answers 401 without a usable session.
func TestTransactionsNeedSession(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	it := e.mustCreateTx(tok, e.categoryID(tok, "Food"), nil)
	before := e.txRows(a.id)
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/transactions", ""},
		{http.MethodGet, "/api/transactions?month=2026-01", ""},
		{http.MethodPost, "/api/transactions", txBody(t, e.categoryID(tok, "Food"), nil)},
		{http.MethodPatch, "/api/transactions/" + it.ID.String(), `{"amount":"1"}`},
		{http.MethodDelete, "/api/transactions/" + it.ID.String(), ""},
	}
	for _, r := range routes {
		for _, bad := range []string{"", accountdomain.NewToken().Plain} {
			rec := e.do(r.method, r.path, bad, r.body)
			if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "unauthenticated" {
				t.Errorf("%s %s with token %q: %d %s, want 401", r.method, r.path, bad, rec.Code, rec.Body.String())
			}
		}
	}
	if got := e.txRows(a.id); !slices.Equal(got, before) {
		t.Error("an unauthenticated request changed the transactions")
	}
}

// Removing a user (the operator endpoint, T6) removes their transactions and nobody else's.
func TestRemoveUserRemovesTransactions(t *testing.T) {
	e := newAPIEnv(t)
	op := e.newAccount(accountOpts{operator: true})
	b, c := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	bTok, cTok := e.freshSession(b.id), e.freshSession(c.id)
	for range 3 {
		e.mustCreateTx(bTok, e.categoryID(bTok, "Food"), nil)
	}
	e.mustCreateTx(cTok, e.categoryID(cTok, "Food"), nil)
	if rec := e.removeUser(e.freshSession(op.id), b.id.String()); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
	}
	if n := e.count("select count(*) from transactions where owner_id = $1", b.id); n != 0 {
		t.Errorf("B still has %d transactions", n)
	}
	if n := e.count("select count(*) from transactions where owner_id = $1", c.id); n != 1 {
		t.Errorf("C has %d transactions, want 1", n)
	}
}

// No amount, merchant, note or date reaches the log: not from a create, an update, a list query
// or a failing request (ADR-0065, ADR-0067).
func TestTransactionLogsNoValues(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food := e.categoryID(tok, "Food")

	it := e.mustCreateTx(tok, food, map[string]any{
		"amount": "98765.43", "occurred_on": "2003-03-14", "merchant": "Merchant qxvzm", "note": "Note qxvzn",
	})
	e.mustPatchTx(tok, it.ID, `{"amount":"87654.32","merchant":"Shop qxvzs","note":"Other qxvzo","occurred_on":"2004-04-15"}`)
	for _, body := range []string{
		txBody(t, food, map[string]any{"amount": "76543.219", "merchant": "Bad qxvzb"}),
		txBody(t, food, map[string]any{"amount": "65432.10", "occurred_on": "2005-02-30", "note": "Bad qxvzd"}),
		txBody(t, food, map[string]any{"amount": "54321.00", "merchant": "Bad\tqxvzc"}),
		txBody(t, uuid.New(), map[string]any{"amount": "43210.98", "occurred_on": "2006-06-16"}),
		`{"amount":"32109.87","merchant":"Malformed qxvzj"`,
	} {
		if rec := e.createTx(tok, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("create %s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := e.patchTx(tok, it.ID.String(), `{"amount":"21098.765"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad PATCH: %d", rec.Code)
	}
	e.listTxs(tok, query("from", "2007-07-17", "to", "2008-08-18"))
	if rec := e.do(http.MethodGet, "/api/transactions?month=2009-09&from=2009-09-19", tok, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad list: %d", rec.Code)
	}

	logs := e.logs.String()
	if !strings.Contains(logs, `"route":"/api/transactions"`) {
		t.Fatalf("no request lines were captured:\n%s", logs)
	}
	for _, secret := range []string{
		"98765", "87654", "76543", "65432", "54321", "43210", "32109", "21098", "qxvz",
		"2003-03-14", "2004-04-15", "2005-02-30", "2006-06-16", "2007-07-17", "2008-08-18", "2009-09",
	} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain %q:\n%s", secret, logs)
		}
	}
}
