package learn

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categoriesdomain "github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	categorizationupsertport "github.com/tarlrsk/expense-tracker/api/internal/categorization/port/upsert"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	upsertFn   func(ctx context.Context, r categorizationupsertport.NewRule) (domain.Rule, error)
	categoryFn func(ctx context.Context, ownerID, id uuid.UUID) (categoriesdomain.Category, bool, error)
)

func (f upsertFn) Upsert(ctx context.Context, r categorizationupsertport.NewRule) (domain.Rule, error) {
	return f(ctx, r)
}

func (f categoryFn) Find(ctx context.Context, o, id uuid.UUID) (categoriesdomain.Category, bool, error) {
	return f(ctx, o, id)
}

func TestExecute(t *testing.T) {
	user, cat := uuid.New(), uuid.New()
	boom := errors.New("boom")
	tests := []struct {
		name         string
		merchant     string
		category     *categoriesdomain.Category // nil: not found
		categoryErr  error
		upsertErr    error
		wantErr      error
		wantLearnt   bool
		wantKey      string
		wantMerchant string
		wantCalls    []string
		wantTx       []txtest.Outcome
	}{
		{
			name: "active category: the rule is written with the key and the name as written", merchant: " 7-Eleven ",
			category: &categoriesdomain.Category{ID: cat}, wantLearnt: true, wantKey: "7eleven", wantMerchant: "7-Eleven",
			wantCalls: []string{"category", "upsert"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "Thai merchant", merchant: "ข้าว มัน ไก่", category: &categoriesdomain.Category{ID: cat}, wantLearnt: true,
			wantKey: "ข้าวมันไก่", wantMerchant: "ข้าว มัน ไก่", wantCalls: []string{"category", "upsert"},
			wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "archived category: nothing written", merchant: "Grab", category: &categoriesdomain.Category{ID: cat, Archived: true},
			wantCalls: []string{"category"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "unknown category: nothing written", merchant: "Grab",
			wantCalls: []string{"category"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{name: "no merchant: no transaction", merchant: ""},
		{name: "a merchant without a key: no transaction", merchant: "---"},
		{name: "a key over 100 characters: no transaction", merchant: strings.Repeat("ﷺ", 7)},
		{name: "a merchant over 100 characters: no transaction", merchant: strings.Repeat("a ", 50) + "b"},
		{
			name: "the category read fails: rolled back", merchant: "Grab", categoryErr: boom, wantErr: boom,
			wantCalls: []string{"category"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "the upsert fails: rolled back", merchant: "Grab", category: &categoriesdomain.Category{ID: cat},
			upsertErr: boom, wantErr: boom, wantKey: "grab", wantMerchant: "Grab",
			wantCalls: []string{"category", "upsert"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			var calls []string
			call := func(ctx context.Context, name string, owner uuid.UUID) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user {
					t.Errorf("%s outside the caller's user transaction: %v", name, err)
				}
				if owner != user {
					t.Errorf("%s for %s, want the caller", name, owner)
				}
				calls = append(calls, name)
			}
			p := New(utx, Ports{
				Category: categoryFn(func(ctx context.Context, o, id uuid.UUID) (categoriesdomain.Category, bool, error) {
					call(ctx, "category", o)
					if id != cat {
						t.Errorf("category %s, want %s", id, cat)
					}
					if tt.category == nil {
						return categoriesdomain.Category{}, false, tt.categoryErr
					}
					return *tt.category, true, nil
				}),
				Upsert: upsertFn(func(ctx context.Context, r categorizationupsertport.NewRule) (domain.Rule, error) {
					call(ctx, "upsert", r.OwnerID)
					want := categorizationupsertport.NewRule{OwnerID: user, MerchantKey: tt.wantKey, Merchant: tt.wantMerchant, CategoryID: cat}
					if r != want {
						t.Errorf("upsert %+v, want %+v", r, want)
					}
					return domain.Rule{MerchantKey: r.MerchantKey, Merchant: r.Merchant, CategoryID: r.CategoryID}, tt.upsertErr
				}),
			})
			resp, err := p.Execute(t.Context(), Request{UserID: user, Merchant: tt.merchant, CategoryID: cat})
			if !errors.Is(err, tt.wantErr) || (err == nil) != (tt.wantErr == nil) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err != nil && apperr.KindOf(err) != apperr.Internal {
				t.Errorf("error kind = %q, want internal", apperr.KindOf(err))
			}
			if resp.Learnt != tt.wantLearnt {
				t.Errorf("learnt = %v, want %v", resp.Learnt, tt.wantLearnt)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
			var outcomes []txtest.Outcome
			for _, r := range utx.Records() {
				outcomes = append(outcomes, r.Outcome)
			}
			if !slices.Equal(outcomes, tt.wantTx) {
				t.Errorf("transactions = %v, want %v", outcomes, tt.wantTx)
			}
		})
	}
}

// Inside the caller's open transaction, learn joins it: no transaction of its own, and its
// failure rolls the caller's back.
func TestExecuteJoins(t *testing.T) {
	user, cat := uuid.New(), uuid.New()
	for _, fail := range []bool{false, true} {
		utx := txtest.New()
		p := New(utx, Ports{
			Category: categoryFn(func(context.Context, uuid.UUID, uuid.UUID) (categoriesdomain.Category, bool, error) {
				return categoriesdomain.Category{ID: cat}, true, nil
			}),
			Upsert: upsertFn(func(context.Context, categorizationupsertport.NewRule) (domain.Rule, error) {
				if fail {
					return domain.Rule{}, errors.New("boom")
				}
				return domain.Rule{}, nil
			}),
		})
		err := utx.WithUserTx(t.Context(), user, func(ctx context.Context) error {
			_, _ = p.Execute(ctx, Request{UserID: user, Merchant: "Grab", CategoryID: cat})
			return nil // even when the caller ignores learn's error
		})
		want := []txtest.Record{{Role: tx.RoleUser, UserID: user, Outcome: txtest.Committed}}
		if fail {
			want[0].Outcome = txtest.RolledBack
		}
		if got := utx.Records(); !slices.Equal(got, want) {
			t.Errorf("fail %v: transactions = %+v, want %+v", fail, got, want)
		}
		if fail != (err != nil) {
			t.Errorf("fail %v: error = %v", fail, err)
		}
	}
}
