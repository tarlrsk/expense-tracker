package matchrules

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	categorizationdomain "github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type ruleFn func(ctx context.Context, ownerID uuid.UUID, key string) (categorizationdomain.Rule, bool, error)

func (f ruleFn) Find(ctx context.Context, o uuid.UUID, key string) (categorizationdomain.Rule, bool, error) {
	return f(ctx, o, key)
}

func TestExecute(t *testing.T) {
	user := uuid.New()
	food, transport := uuid.New(), uuid.New()
	rules := map[string]categorizationdomain.Rule{
		"grab":    {MerchantKey: "grab", Merchant: "Grab", CategoryID: transport},
		"7eleven": {MerchantKey: "7eleven", Merchant: "7-Eleven", CategoryID: food},
	}
	boom := errors.New("boom")
	tests := []struct {
		name      string
		merchants []string
		findErr   error
		wantErr   error
		want      []uuid.UUID // the category matched per merchant; uuid.Nil for none
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "each merchant by its key, in order", merchants: []string{"GRAB", "coffee", "7 Eleven"},
			want: []uuid.UUID{transport, uuid.Nil, food}, wantCalls: []string{"grab", "coffee", "7eleven"},
			wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "a key is looked up once", merchants: []string{"grab", "Grab", "g r a b"},
			want: []uuid.UUID{transport, transport, transport}, wantCalls: []string{"grab"},
			wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "merchants without a key: no transaction", merchants: []string{"", "---"},
			want: []uuid.UUID{uuid.Nil, uuid.Nil},
		},
		{name: "no merchants: no transaction", merchants: nil, want: []uuid.UUID{}},
		{
			name: "the lookup fails: rolled back", merchants: []string{"grab"}, findErr: boom, wantErr: boom,
			wantCalls: []string{"grab"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			var calls []string
			p := New(utx, Ports{Rule: ruleFn(func(ctx context.Context, o uuid.UUID, key string) (categorizationdomain.Rule, bool, error) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user || o != user {
					t.Errorf("find %q outside the caller's user transaction (%v) or for %s", key, err, o)
				}
				calls = append(calls, key)
				if tt.findErr != nil {
					return categorizationdomain.Rule{}, false, tt.findErr
				}
				r, found := rules[key]
				return r, found, nil
			})})
			resp, err := p.Execute(t.Context(), Request{UserID: user, Merchants: tt.merchants})
			if !errors.Is(err, tt.wantErr) || (err == nil) != (tt.wantErr == nil) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				got := make([]uuid.UUID, 0, len(resp.Matches))
				for _, m := range resp.Matches {
					if m.Found != (m.Rule.CategoryID != uuid.Nil) {
						t.Errorf("match %+v", m)
					}
					got = append(got, m.Rule.CategoryID)
				}
				if !slices.Equal(got, tt.want) {
					t.Errorf("matched %v, want %v", got, tt.want)
				}
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("lookups = %q, want %q", calls, tt.wantCalls)
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
