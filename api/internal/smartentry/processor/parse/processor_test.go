package parse

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categorizationdomain "github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type ruleFn func(ctx context.Context, ownerID uuid.UUID, key string) (categorizationdomain.Rule, bool, error)

func (f ruleFn) Find(ctx context.Context, o uuid.UUID, key string) (categorizationdomain.Rule, bool, error) {
	return f(ctx, o, key)
}

// show writes an item in short: "text | amount | date | merchant | category | resolved".
func show(it Item, cats map[uuid.UUID]string) string {
	amount := "-"
	if it.HasAmount {
		amount = it.Amount.String()
	}
	cat := "-"
	if it.CategoryID != uuid.Nil {
		cat = cats[it.CategoryID]
	}
	resolved := "no"
	if it.Resolved {
		resolved = "resolved"
	}
	return it.Text + " | " + amount + " | " + it.OccurredOn.String() + " | " + it.Merchant + " | " + cat + " | " + resolved
}

func TestExecute(t *testing.T) {
	user := uuid.New()
	food, transport := uuid.New(), uuid.New()
	cats := map[uuid.UUID]string{food: "food", transport: "transport"}
	rules := map[string]categorizationdomain.Rule{
		"grab":       {MerchantKey: "grab", Merchant: "Grab", CategoryID: transport},
		"ข้าวมันไก่": {MerchantKey: "ข้าวมันไก่", Merchant: "ข้าว มัน ไก่", CategoryID: food},
		"7eleven":    {MerchantKey: "7eleven", Merchant: "7-Eleven", CategoryID: food},
	}
	boom := errors.New("boom")
	tests := []struct {
		name      string
		text      string
		findErr   error
		wantErr   error
		want      []string
		wantCalls []string // the keys looked up, in order
		wantTx    []txtest.Outcome
	}{
		{
			name: "a rule resolves the item with the rule's category and stored name", text: "grab 145",
			want:      []string{"grab 145 | 145.00 | 2026-10-03 | Grab | transport | resolved"},
			wantCalls: []string{"grab"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "Thai, with the date word", text: "ข้าวมันไก่ 60 เมื่อวาน",
			want:      []string{"ข้าวมันไก่ 60 เมื่อวาน | 60.00 | 2026-10-02 | ข้าว มัน ไก่ | food | resolved"},
			wantCalls: []string{"ข้าวมันไก่"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "7 ELEVEN is two numbers: incomplete, not looked up", text: "7 ELEVEN ฿60",
			want: []string{"7 ELEVEN ฿60 | - | 2026-10-03 | 7 ELEVEN ฿60 | - | no"},
		},
		{
			name: "a hyphenated name is looked up by its key", text: "7-ELEVEN ฿60",
			want:      []string{"7-ELEVEN ฿60 | 60.00 | 2026-10-03 | 7-Eleven | food | resolved"},
			wantCalls: []string{"7eleven"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "no rule: not resolved, merchant as typed", text: "cafe amazon 75",
			want:      []string{"cafe amazon 75 | 75.00 | 2026-10-03 | cafe amazon | - | no"},
			wantCalls: []string{"cafeamazon"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "mixed: one resolved, one not, one incomplete; a middle date word is its own only", text: "coffee 60, grab 145 yesterday, rice 1k",
			want: []string{
				"coffee 60 | 60.00 | 2026-10-03 | coffee | - | no",
				"grab 145 yesterday | 145.00 | 2026-10-02 | Grab | transport | resolved",
				"rice 1k | - | 2026-10-03 | rice 1k | - | no",
			},
			wantCalls: []string{"coffee", "grab"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "a key is looked up once for several items", text: "grab 60, Grab 145, coffee 50, GRAB 20",
			want: []string{
				"grab 60 | 60.00 | 2026-10-03 | Grab | transport | resolved",
				"Grab 145 | 145.00 | 2026-10-03 | Grab | transport | resolved",
				"coffee 50 | 50.00 | 2026-10-03 | coffee | - | no",
				"GRAB 20 | 20.00 | 2026-10-03 | Grab | transport | resolved",
			},
			wantCalls: []string{"grab", "coffee"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "incomplete items are never looked up: no transaction", text: "grab 1k, grab, 145, grab 60 baht baht, 7 eleven 60",
			want: []string{
				"grab 1k | - | 2026-10-03 | grab 1k | - | no",
				"grab | - | 2026-10-03 | grab | - | no",
				"145 | 145.00 | 2026-10-03 |  | - | no",
				"grab 60 baht baht | 60.00 | 2026-10-03 | grab baht | - | no",
				"7 eleven 60 | - | 2026-10-03 | 7 eleven 60 | - | no",
			},
		},
		{
			name: "an item whose day is undecided is not looked up", text: "yesterday, grab 60, today",
			want: []string{"grab 60 | 60.00 |  | grab | - | no"},
		},
		{
			name: "a complete item whose merchant has no key: no transaction", text: "--- 60",
			want: []string{"--- 60 | 60.00 | 2026-10-03 | --- | - | no"},
		},
		{name: "empty text: no items, no transaction", text: "  ", want: nil},
		{
			name: "the lookup fails: rolled back", text: "coffee 60, grab 145", findErr: boom, wantErr: boom,
			wantCalls: []string{"coffee"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
	}
	// 17:30 UTC on 2 October 2026 is 3 October in Bangkok.
	now := func() time.Time { return time.Date(2026, time.October, 2, 17, 30, 0, 0, time.UTC) }
	bangkok := time.FixedZone("ICT", 7*60*60)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			var calls []string
			p := New(utx, now, bangkok, Ports{
				Rule: ruleFn(func(ctx context.Context, o uuid.UUID, key string) (categorizationdomain.Rule, bool, error) {
					if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user {
						t.Errorf("find %q outside the caller's user transaction: %v", key, err)
					}
					if o != user {
						t.Errorf("find %q for %s, want the caller", key, o)
					}
					calls = append(calls, key)
					if tt.findErr != nil {
						return categorizationdomain.Rule{}, false, tt.findErr
					}
					r, found := rules[key]
					return r, found, nil
				}),
			})
			resp, err := p.Execute(t.Context(), Request{UserID: user, Text: tt.text})
			if !errors.Is(err, tt.wantErr) || (err == nil) != (tt.wantErr == nil) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				if apperr.KindOf(err) != apperr.Internal {
					t.Errorf("error kind = %q, want internal", apperr.KindOf(err))
				}
				if resp.Items != nil {
					t.Errorf("items %+v with an error", resp.Items)
				}
			}
			var got []string
			for _, it := range resp.Items {
				got = append(got, show(it, cats))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("items:\n got %q\nwant %q", got, tt.want)
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
