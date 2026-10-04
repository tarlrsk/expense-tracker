package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categorizationcreatetransactionorch "github.com/tarlrsk/expense-tracker/api/internal/categorization/orchestrator/createtransaction"
	categorizationupdatetransactionorch "github.com/tarlrsk/expense-tracker/api/internal/categorization/orchestrator/updatetransaction"
	categorizationlearnproc "github.com/tarlrsk/expense-tracker/api/internal/categorization/processor/learn"
	transactionsreg "github.com/tarlrsk/expense-tracker/api/internal/registry/transactions"
	transactionscreateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/create"
	transactionsupdateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/update"
)

// API-level tests of merchant-rule learning on POST and PATCH /api/transactions (PLAN-0003 T2;
// ADR-0076, ADR-0078).

// merchantRules is every rule of the user, read as the superuser, ordered by key: "key | merchant
// | category | updated_at". Two equal snapshots mean nothing was written (the trigger moves
// updated_at on every update).
func (e *apiEnv) merchantRules(userID uuid.UUID) []string {
	e.t.Helper()
	rows, err := e.super.QueryContext(e.t.Context(),
		"select merchant_key, merchant, category_id, updated_at from merchant_rules where owner_id = $1 order by merchant_key", userID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var (
			key, merchant string
			category      uuid.UUID
			updatedAt     time.Time
		)
		if err := rows.Scan(&key, &merchant, &category, &updatedAt); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s | %s | %s | %d", key, merchant, category, updatedAt.UnixMicro()))
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// merchantRule returns the merchant and category of the user's rule for key; ok is false when
// there is none.
func (e *apiEnv) merchantRule(userID uuid.UUID, key string) (merchant string, category uuid.UUID, ok bool) {
	e.t.Helper()
	for _, r := range e.merchantRules(userID) {
		parts := strings.Split(r, " | ")
		if parts[0] == key {
			return parts[1], uuid.MustParse(parts[2]), true
		}
	}
	return "", uuid.Nil, false
}

// assertRule fails unless the user's rule for key has the merchant and category.
func (e *apiEnv) assertRule(userID uuid.UUID, key, merchant string, category uuid.UUID) {
	e.t.Helper()
	m, c, ok := e.merchantRule(userID, key)
	if !ok || m != merchant || c != category {
		e.t.Errorf("rule %q = %q, %s, found %v; want %q, %s (all: %v)", key, m, c, ok, merchant, category, e.merchantRules(userID))
	}
}

// assertRulesUnchanged fails unless the user's rules are exactly before.
func (e *apiEnv) assertRulesUnchanged(userID uuid.UUID, before []string, what string) {
	e.t.Helper()
	if got := e.merchantRules(userID); !slices.Equal(got, before) {
		e.t.Errorf("%s changed the rules:\n before %v\n after  %v", what, before, got)
	}
}

func TestMerchantRulesLearntOnCreate(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")

	t.Run("a create with a merchant writes the rule: key, the name as saved, the category", func(t *testing.T) {
		e := e.with(t)
		e.mustCreateTx(tok, food, map[string]any{"merchant": "  7-Eleven  "})
		e.assertRule(a.id, "7eleven", "7-Eleven", food)
	})

	t.Run("the latest choice wins: one rule per key, name and category replaced", func(t *testing.T) {
		e := e.with(t)
		e.mustCreateTx(tok, transport, map[string]any{"merchant": "7 ELEVEN"})
		e.assertRule(a.id, "7eleven", "7 ELEVEN", transport)
		if n := e.count("select count(*) from merchant_rules where owner_id = $1 and merchant_key = '7eleven'", a.id); n != 1 {
			t.Errorf("rows for 7eleven = %d, want 1", n)
		}
	})

	t.Run("Thai: spaces are not part of the key, marks are", func(t *testing.T) {
		e := e.with(t)
		e.mustCreateTx(tok, food, map[string]any{"merchant": "ข้าว มัน ไก่"})
		e.assertRule(a.id, "ข้าวมันไก่", "ข้าว มัน ไก่", food)
		e.mustCreateTx(tok, transport, map[string]any{"merchant": "ข้าวมันไก่"})
		e.assertRule(a.id, "ข้าวมันไก่", "ข้าวมันไก่", transport)
	})

	t.Run("no merchant, or one without a key, teaches nothing", func(t *testing.T) {
		e := e.with(t)
		before := e.merchantRules(a.id)
		e.mustCreateTx(tok, food, nil)
		e.mustCreateTx(tok, food, map[string]any{"merchant": "   "})
		e.mustCreateTx(tok, food, map[string]any{"merchant": "---"})
		e.assertRulesUnchanged(a.id, before, "creates without a merchant key")
	})

	t.Run("a retry with the same id (200) teaches nothing", func(t *testing.T) {
		e := e.with(t)
		id := newTxID(t)
		if rec := e.createTx(tok, txBody(t, food, map[string]any{"id": id, "merchant": "Grab"})); rec.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
		e.assertRule(a.id, "grab", "Grab", food)
		before := e.merchantRules(a.id)
		for _, fields := range []map[string]any{
			{"id": id, "merchant": "Grab"},
			{"id": id, "merchant": "GRAB", "category_id": transport.String()},
		} {
			if rec := e.createTx(tok, txBody(t, food, fields)); rec.Code != http.StatusOK {
				t.Fatalf("retry %v: %d %s", fields, rec.Code, rec.Body.String())
			}
		}
		e.assertRulesUnchanged(a.id, before, "a retry")
	})

	t.Run("a refused create teaches nothing", func(t *testing.T) {
		e := e.with(t)
		pets := e.mustCreateCategory(tok, "Pets")
		e.mustPatchCategory(tok, pets.ID, `{"archived":true}`)
		before := e.merchantRules(a.id)
		for _, body := range []string{
			txBody(t, uuid.New(), map[string]any{"merchant": "Vet"}),
			txBody(t, pets.ID, map[string]any{"merchant": "Vet"}),
			txBody(t, food, map[string]any{"merchant": "Vet", "amount": "0"}),
		} {
			if rec := e.createTx(tok, body); rec.Code != http.StatusBadRequest {
				t.Errorf("create %s: %d %s, want 400", body, rec.Code, rec.Body.String())
			}
		}
		e.assertRulesUnchanged(a.id, before, "a refused create")
	})
}

func TestMerchantRulesLearntOnUpdate(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food, transport, shopping := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport"), e.categoryID(tok, "Shopping")
	it := e.mustCreateTx(tok, food, map[string]any{"merchant": "Grab", "amount": "145", "note": "n"})
	e.assertRule(a.id, "grab", "Grab", food)

	t.Run("a change of category teaches the new category", func(t *testing.T) {
		e := e.with(t)
		e.mustPatchTx(tok, it.ID, `{"category_id":"`+transport.String()+`"}`)
		e.assertRule(a.id, "grab", "Grab", transport)
	})

	t.Run("a change of merchant teaches the new merchant's rule; the old one stays", func(t *testing.T) {
		e := e.with(t)
		e.mustPatchTx(tok, it.ID, `{"merchant":" Grab Food "}`)
		e.assertRule(a.id, "grabfood", "Grab Food", transport)
		e.assertRule(a.id, "grab", "Grab", transport)
	})

	t.Run("a new spelling of the same key replaces the name", func(t *testing.T) {
		e := e.with(t)
		e.mustPatchTx(tok, it.ID, `{"merchant":"GRAB-FOOD","category_id":"`+shopping.String()+`"}`)
		e.assertRule(a.id, "grabfood", "GRAB-FOOD", shopping)
	})

	t.Run("a change of only amount, date, note or currency, or no change, teaches nothing", func(t *testing.T) {
		e := e.with(t)
		before := e.merchantRules(a.id)
		for _, body := range []string{
			`{"amount":"99"}`, `{"occurred_on":"2026-01-02"}`, `{"note":"other"}`, `{"currency":"THB"}`,
			`{"merchant":"GRAB-FOOD"}`, `{"merchant":" GRAB-FOOD ","category_id":"` + shopping.String() + `"}`,
		} {
			e.mustPatchTx(tok, it.ID, body)
		}
		e.assertRulesUnchanged(a.id, before, "PATCH without a category or merchant change")
	})

	t.Run("clearing the merchant leaves the rules as they are; so do changes without a merchant", func(t *testing.T) {
		e := e.with(t)
		before := e.merchantRules(a.id)
		e.mustPatchTx(tok, it.ID, `{"merchant":""}`)
		e.mustPatchTx(tok, it.ID, `{"category_id":"`+food.String()+`"}`)
		e.assertRulesUnchanged(a.id, before, "clearing the merchant")
	})

	t.Run("a transaction whose category is archived teaches nothing", func(t *testing.T) {
		e := e.with(t)
		pets := e.mustCreateCategory(tok, "Pets")
		vet := e.mustCreateTx(tok, pets.ID, map[string]any{"merchant": "Vet"})
		e.assertRule(a.id, "vet", "Vet", pets.ID)
		e.mustPatchCategory(tok, pets.ID, `{"archived":true}`)
		before := e.merchantRules(a.id)
		if got := e.mustPatchTx(tok, vet.ID, `{"merchant":"Vet Clinic"}`); got.CategoryID != pets.ID {
			t.Fatalf("PATCH: %+v", got)
		}
		e.assertRulesUnchanged(a.id, before, "a merchant change on an archived category")
	})

	t.Run("a refused PATCH teaches nothing", func(t *testing.T) {
		e := e.with(t)
		before := e.merchantRules(a.id)
		for _, body := range []string{
			`{"merchant":"Lineman","category_id":"` + uuid.NewString() + `"}`, `{"merchant":"Lineman","amount":"0"}`,
		} {
			assertBadRequest(t, "PATCH "+body, e.patchTx(tok, it.ID.String(), body), "")
		}
		if rec := e.patchTx(tok, newTxID(t), `{"merchant":"Lineman"}`); rec.Code != http.StatusNotFound {
			t.Errorf("PATCH unknown id: %d", rec.Code)
		}
		e.assertRulesUnchanged(a.id, before, "a refused PATCH")
	})

	t.Run("deleting a transaction leaves the rules as they are", func(t *testing.T) {
		e := e.with(t)
		before := e.merchantRules(a.id)
		if rec := e.deleteTx(tok, it.ID.String()); rec.Code != http.StatusNoContent {
			t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
		}
		e.assertRulesUnchanged(a.id, before, "a delete")
	})
}

// The save and the rule are one transaction (ADR-0078): when learning fails, nothing of the save
// is stored. The real create and update processors run on the test database; only learn fails.
func TestMerchantRuleFailureRollsBackTheSave(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")
	it := e.mustCreateTx(tok, food, map[string]any{"merchant": "Grab"})
	txBefore, rulesBefore := e.txRows(a.id), e.merchantRules(a.id)

	boom := errors.New("learn failed")
	failing := learnFunc(func(context.Context, categorizationlearnproc.Request) (categorizationlearnproc.Response, error) {
		return categorizationlearnproc.Response{}, boom
	})

	create := categorizationcreatetransactionorch.New(e.deps.UserTx, transactionsreg.NewCreate(e.deps), failing)
	id := newTxID(t)
	_, err := create.Execute(t.Context(), transactionscreateproc.Request{
		UserID: a.id, ID: id, Amount: "10", OccurredOn: "2026-01-15", CategoryID: food.String(), Merchant: "Lineman",
	})
	if !errors.Is(err, boom) || apperr.KindOf(err) != apperr.Internal {
		t.Fatalf("create error = %v, want the learn error (internal)", err)
	}
	if n := e.count("select count(*) from transactions where id = $1", id); n != 0 {
		t.Errorf("the transaction was stored although learning failed")
	}

	update := categorizationupdatetransactionorch.New(e.deps.UserTx, transactionsreg.NewUpdate(e.deps), failing)
	cat := transport.String()
	_, err = update.Execute(t.Context(), transactionsupdateproc.Request{UserID: a.id, ID: it.ID, CategoryID: &cat})
	if !errors.Is(err, boom) {
		t.Fatalf("update error = %v, want the learn error", err)
	}
	if got := e.txRows(a.id); !slices.Equal(got, txBefore) {
		t.Error("the update was stored although learning failed")
	}
	e.assertRulesUnchanged(a.id, rulesBefore, "a failed learn")
}

type learnFunc func(context.Context, categorizationlearnproc.Request) (categorizationlearnproc.Response, error)

func (f learnFunc) Execute(ctx context.Context, r categorizationlearnproc.Request) (categorizationlearnproc.Response, error) {
	return f(ctx, r)
}

func TestMerchantRulesCrossUser(t *testing.T) {
	e := newAPIEnv(t)
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)
	aFood, aTransport := e.categoryID(aTok, "Food"), e.categoryID(aTok, "Transport")
	bFood, bShopping := e.categoryID(bTok, "Food"), e.categoryID(bTok, "Shopping")

	bTx := e.mustCreateTx(bTok, bShopping, map[string]any{"merchant": "B private qxvz"})
	e.mustCreateTx(bTok, bShopping, map[string]any{"merchant": "Grab"})
	bRules := e.merchantRules(b.id)

	t.Run("A and B hold the same key with different categories", func(t *testing.T) {
		e := e.with(t)
		e.mustCreateTx(aTok, aFood, map[string]any{"merchant": "GRAB"})
		e.assertRule(a.id, "grab", "GRAB", aFood)
		e.assertRule(b.id, "grab", "Grab", bShopping)
	})

	t.Run("A's creates and changes never create or change B's rules", func(t *testing.T) {
		e := e.with(t)
		it := e.mustCreateTx(aTok, aFood, map[string]any{"merchant": "Grab"})
		e.mustPatchTx(aTok, it.ID, `{"category_id":"`+aTransport.String()+`"}`)
		e.mustPatchTx(aTok, it.ID, `{"merchant":"B private qxvz"}`)
		e.assertRule(a.id, "grab", "Grab", aTransport)
		e.assertRule(a.id, "bprivateqxvz", "B private qxvz", aTransport)
		e.assertRulesUnchanged(b.id, bRules, "A's saves")
	})

	t.Run("A's refused requests on B's transaction or category write no rule for anyone", func(t *testing.T) {
		e := e.with(t)
		aRules := e.merchantRules(a.id)
		if rec := e.patchTx(aTok, bTx.ID.String(), `{"merchant":"Taken","category_id":"`+aFood.String()+`"}`); rec.Code != http.StatusNotFound {
			t.Errorf("A's PATCH on B's id: %d %s", rec.Code, rec.Body.String())
		}
		if rec := e.createTx(aTok, txBody(t, aFood, map[string]any{"id": bTx.ID.String(), "merchant": "Taken"})); rec.Code != http.StatusBadRequest {
			t.Errorf("A's create with B's id: %d %s", rec.Code, rec.Body.String())
		}
		if rec := e.createTx(aTok, txBody(t, bFood, map[string]any{"merchant": "Taken"})); rec.Code != http.StatusBadRequest {
			t.Errorf("A's create in B's category: %d %s", rec.Code, rec.Body.String())
		}
		e.assertRulesUnchanged(a.id, aRules, "A's refused requests")
		e.assertRulesUnchanged(b.id, bRules, "A's refused requests")
		if n := e.count("select count(*) from merchant_rules where category_id = $1 and owner_id <> $2", bFood, b.id); n != 0 {
			t.Errorf("%d rules of another user point at B's category", n)
		}
	})

	// No endpoint reads rules yet (T3 and T5 add the lookup); a save's response is the saved
	// transaction only, so B's rule for the same key cannot show through it.
	t.Run("A's saves of B's key answer with A's transaction only", func(t *testing.T) {
		e := e.with(t)
		for _, rec := range []string{
			e.createTx(aTok, txBody(t, aFood, map[string]any{"merchant": "grab"})).Body.String(),
			e.getTxs(aTok, nil).Body.String(),
		} {
			if strings.Contains(rec, b.id.String()) || strings.Contains(rec, bShopping.String()) {
				t.Errorf("A's response shows B's data: %s", rec)
			}
		}
		e.assertRule(a.id, "grab", "grab", aFood)
		e.assertRule(b.id, "grab", "Grab", bShopping)
	})
}
