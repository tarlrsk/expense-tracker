package app

import (
	"testing"
	"time"

	"github.com/google/uuid"

	smartentryreg "github.com/tarlrsk/expense-tracker/api/internal/registry/smartentry"
	smartentryparseproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/parse"
)

// Tests of the local parser (PLAN-0003 T3) as wired by registry/smartentry, on the Docker test
// database: the rule lookup runs as app_user under row-level security. There is no endpoint yet
// (T5), so the use case is called directly.

// addRule inserts a merchant rule for the user as the superuser.
func (e *apiEnv) addRule(userID uuid.UUID, key, merchant string, category uuid.UUID) {
	e.t.Helper()
	e.exec("insert into merchant_rules (owner_id, merchant_key, merchant, category_id) values ($1, $2, $3, $4)",
		userID, key, merchant, category)
}

// parseEntry runs the local parser for the user and returns its items.
func (e *apiEnv) parseEntry(userID uuid.UUID, text string) []smartentryparseproc.Item {
	e.t.Helper()
	resp, err := smartentryreg.NewParse(e.deps).Execute(e.t.Context(), smartentryparseproc.Request{UserID: userID, Text: text})
	if err != nil {
		e.t.Fatalf("parse %q: %v", text, err)
	}
	return resp.Items
}

// parsed is the part of an item these tests check.
type parsed struct {
	merchant string
	category uuid.UUID
	resolved bool
}

func (e *apiEnv) assertParsed(userID uuid.UUID, text string, want ...parsed) {
	e.t.Helper()
	items := e.parseEntry(userID, text)
	if len(items) != len(want) {
		e.t.Fatalf("parse %q: %d items %+v, want %d", text, len(items), items, len(want))
	}
	for i, w := range want {
		got := parsed{merchant: items[i].Merchant, category: items[i].CategoryID, resolved: items[i].Resolved}
		if got != w {
			e.t.Errorf("parse %q item %d = %+v, want %+v", text, i, got, w)
		}
	}
}

func TestSmartEntryParse(t *testing.T) {
	// 17:30 UTC on 2 October 2026 is 3 October in Bangkok.
	e := newAPIEnv(t, withClock(time.Date(2026, time.October, 2, 17, 30, 0, 0, time.UTC)))
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")
	e.addRule(a.id, "grab", "Grab", transport)
	e.addRule(a.id, "ข้าวมันไก่", "ข้าว มัน ไก่", food)

	t.Run("a rule resolves the item with its category and stored name; the day is Bangkok's", func(t *testing.T) {
		e := e.with(t)
		items := e.parseEntry(a.id, "grab 145")
		if len(items) != 1 {
			t.Fatalf("items %+v", items)
		}
		it := items[0]
		if !it.Resolved || it.Merchant != "Grab" || it.CategoryID != transport || !it.HasAmount || it.Amount.String() != "145.00" ||
			it.OccurredOn.String() != "2026-10-03" || it.Text != "grab 145" {
			t.Errorf("item %+v", it)
		}
		e.assertParsed(a.id, "ข้าวมันไก่ 60 เมื่อวาน", parsed{"ข้าว มัน ไก่", food, true})
	})

	t.Run("no rule: not resolved, the merchant as typed", func(t *testing.T) {
		e := e.with(t)
		e.assertParsed(a.id, "cafe amazon 75", parsed{"cafe amazon", uuid.Nil, false})
	})

	t.Run("mixed text: one resolved, one not", func(t *testing.T) {
		e := e.with(t)
		e.assertParsed(a.id, "coffee 60, GRAB 145 yesterday",
			parsed{"coffee", uuid.Nil, false}, parsed{"Grab", transport, true})
	})

	t.Run("incomplete items are not looked up, even when a rule has their key", func(t *testing.T) {
		e := e.with(t)
		e.addRule(a.id, "grab1k", "grab 1k", transport)
		e.addRule(a.id, "7eleven60", "7 eleven 60", food)
		e.assertParsed(a.id, "grab 1k, 7 eleven 60, grab, yesterday grab 60 today",
			parsed{"grab 1k", uuid.Nil, false}, parsed{"7 eleven 60", uuid.Nil, false},
			parsed{"grab", uuid.Nil, false}, parsed{"grab", uuid.Nil, false})
	})

	t.Run("a rule whose category is archived does not resolve", func(t *testing.T) {
		e := e.with(t)
		shopping := e.categoryID(tok, "Shopping")
		e.addRule(a.id, "lazada", "Lazada", shopping)
		e.assertParsed(a.id, "lazada 300", parsed{"Lazada", shopping, true})
		e.exec("update categories set archived = true where id = $1", shopping)
		e.assertParsed(a.id, "lazada 300", parsed{"lazada", uuid.Nil, false})
	})

	t.Run("nothing is written", func(t *testing.T) {
		e := e.with(t)
		before := e.merchantRules(a.id)
		txs := e.count("select count(*) from transactions where owner_id = $1", a.id)
		e.parseEntry(a.id, "grab 145, coffee 60, ข้าวมันไก่ 50")
		e.assertRulesUnchanged(a.id, before, "a parse")
		if n := e.count("select count(*) from transactions where owner_id = $1", a.id); n != txs {
			t.Errorf("transactions %d, want %d", n, txs)
		}
	})
}

// User B's rules never resolve user A's items; each user gets their own rule for the same key.
func TestSmartEntryParseCrossUser(t *testing.T) {
	e := newAPIEnv(t)
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)
	aFood := e.categoryID(aTok, "Food")
	bTransport, bShopping := e.categoryID(bTok, "Transport"), e.categoryID(bTok, "Shopping")
	e.addRule(b.id, "grab", "Grab (B)", bTransport)
	e.addRule(b.id, "bprivateqxvz", "B private qxvz", bShopping)

	t.Run("B's rule does not resolve A's item", func(t *testing.T) {
		e := e.with(t)
		e.assertParsed(a.id, "grab 145, B private qxvz 10",
			parsed{"grab", uuid.Nil, false}, parsed{"B private qxvz", uuid.Nil, false})
	})

	t.Run("each user gets their own rule for the same key", func(t *testing.T) {
		e := e.with(t)
		e.addRule(a.id, "grab", "Grab (A)", aFood)
		e.assertParsed(a.id, "grab 145", parsed{"Grab (A)", aFood, true})
		e.assertParsed(b.id, "grab 145", parsed{"Grab (B)", bTransport, true})
	})

	t.Run("A's archived category does not let B's rule through", func(t *testing.T) {
		e := e.with(t)
		e.exec("update categories set archived = true where id = $1", aFood)
		e.assertParsed(a.id, "grab 145", parsed{"grab", uuid.Nil, false})
		e.assertParsed(b.id, "grab 145", parsed{"Grab (B)", bTransport, true})
	})
}
