package create

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categoriesdomain "github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionsinsertport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/insert"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	findFn     func(ctx context.Context, ownerID, id uuid.UUID) (domain.Transaction, bool, error)
	insertFn   func(ctx context.Context, t transactionsinsertport.NewTransaction) (domain.Transaction, bool, error)
	categoryFn func(ctx context.Context, ownerID, id uuid.UUID) (categoriesdomain.Category, bool, error)
)

func (f findFn) Find(ctx context.Context, o, id uuid.UUID) (domain.Transaction, bool, error) {
	return f(ctx, o, id)
}

func (f insertFn) Insert(ctx context.Context, t transactionsinsertport.NewTransaction) (domain.Transaction, bool, error) {
	return f(ctx, t)
}

func (f categoryFn) Find(ctx context.Context, o, id uuid.UUID) (categoriesdomain.Category, bool, error) {
	return f(ctx, o, id)
}

func ptr(s string) *string { return &s }

func TestExecute(t *testing.T) {
	user, other := uuid.New(), uuid.New()
	id := uuid.Must(uuid.NewV7())
	cat := uuid.New()
	stored := domain.Transaction{ID: id, OwnerID: user, Amount: 999}
	valid := Request{ID: id.String(), Amount: "145", OccurredOn: "2026-01-15", CategoryID: cat.String(), Merchant: " M "}

	type finds []struct {
		t     domain.Transaction
		found bool
	}
	tests := []struct {
		name        string
		req         Request
		finds       finds // answers of the transaction find, in order
		category    *categoriesdomain.Category
		inserted    bool
		insertErr   error
		wantKind    apperr.Kind
		wantMsg     string
		wantCreated bool
		wantCalls   []string
		wantTx      []txtest.Outcome
		// wantSource and wantRaw are the inserted source and raw_input; "" is manual and none.
		wantSource domain.Source
		wantRaw    string
	}{
		{
			name: "created", req: valid, finds: finds{{}}, category: &categoriesdomain.Category{ID: cat}, inserted: true,
			wantCreated: true, wantCalls: []string{"find", "category", "insert"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "the caller's id: stored row, no category check, no insert", req: valid, finds: finds{{stored, true}},
			wantCalls: []string{"find"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "lost a race to the caller's own create: 200 with the winner's row", req: valid, finds: finds{{}, {stored, true}},
			category: &categoriesdomain.Category{ID: cat}, wantCalls: []string{"find", "category", "insert", "find"},
			wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "another user's id (hidden row): the id rule's 400", req: valid, finds: finds{{}, {}},
			category: &categoriesdomain.Category{ID: cat}, wantKind: apperr.InvalidInput, wantMsg: domain.IDRuleMessage,
			wantCalls: []string{"find", "category", "insert", "find"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "a found row authz does not let the caller read is never returned", req: valid,
			finds:    finds{{domain.Transaction{ID: id, OwnerID: other}, true}},
			wantKind: apperr.InvalidInput, wantMsg: domain.IDRuleMessage, wantCalls: []string{"find"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "archived category", req: valid, finds: finds{{}}, category: &categoriesdomain.Category{ID: cat, Archived: true},
			wantKind: apperr.InvalidInput, wantMsg: domain.CategoryMessage, wantCalls: []string{"find", "category"},
			wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "unknown category", req: valid, finds: finds{{}},
			wantKind: apperr.InvalidInput, wantMsg: domain.CategoryMessage, wantCalls: []string{"find", "category"},
			wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "the foreign key refuses the category", req: valid, finds: finds{{}}, category: &categoriesdomain.Category{ID: cat},
			insertErr: transactionsinsertport.ErrCategory, wantKind: apperr.InvalidInput, wantMsg: domain.CategoryMessage,
			wantCalls: []string{"find", "category", "insert"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "database error", req: valid, finds: finds{{}}, category: &categoriesdomain.Category{ID: cat},
			insertErr: errors.New("boom"), wantKind: apperr.Internal,
			wantCalls: []string{"find", "category", "insert"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{name: "bad id: no transaction", req: with(valid, func(r *Request) { r.ID = uuid.NewString() }), wantKind: apperr.InvalidInput, wantMsg: domain.IDRuleMessage},
		{name: "bad amount: no transaction", req: with(valid, func(r *Request) { r.Amount = "1.234" }), wantKind: apperr.InvalidInput, wantMsg: domain.AmountRuleMessage},
		{name: "date too late: no transaction", req: with(valid, func(r *Request) { r.OccurredOn = "2027-10-04" }), wantKind: apperr.InvalidInput, wantMsg: domain.DateRuleMessage},
		{name: "bad category id: no transaction", req: with(valid, func(r *Request) { r.CategoryID = "x" }), wantKind: apperr.InvalidInput, wantMsg: domain.CategoryMessage},
		{name: "bad currency: no transaction", req: with(valid, func(r *Request) { s := "USD"; r.Currency = &s }), wantKind: apperr.InvalidInput, wantMsg: domain.CurrencyMessage},
		{
			name: "source text with raw_input, trimmed", req: with(valid, func(r *Request) { r.Source, r.RawInput = ptr("text"), ptr(" lunch M 145\r\n") }),
			finds: finds{{}}, category: &categoriesdomain.Category{ID: cat}, inserted: true, wantCreated: true,
			wantCalls: []string{"find", "category", "insert"}, wantTx: []txtest.Outcome{txtest.Committed},
			wantSource: domain.SourceText, wantRaw: "lunch M 145",
		},
		{
			name: "source text without raw_input", req: with(valid, func(r *Request) { r.Source = ptr("text") }),
			finds: finds{{}}, category: &categoriesdomain.Category{ID: cat}, inserted: true, wantCreated: true,
			wantCalls: []string{"find", "category", "insert"}, wantTx: []txtest.Outcome{txtest.Committed}, wantSource: domain.SourceText,
		},
		{
			name: "source manual sent: as the default", req: with(valid, func(r *Request) { r.Source = ptr("manual") }),
			finds: finds{{}}, category: &categoriesdomain.Category{ID: cat}, inserted: true, wantCreated: true,
			wantCalls: []string{"find", "category", "insert"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{name: "bad source: no transaction", req: with(valid, func(r *Request) { r.Source = ptr("scan") }), wantKind: apperr.InvalidInput, wantMsg: domain.SourceMessage},
		{
			name: "raw_input with source manual: no transaction", req: with(valid, func(r *Request) { r.Source, r.RawInput = ptr("manual"), ptr("x 5") }),
			wantKind: apperr.InvalidInput, wantMsg: domain.RawInputSourceMessage,
		},
		{
			name: "raw_input without a source: no transaction", req: with(valid, func(r *Request) { r.RawInput = ptr("") }),
			wantKind: apperr.InvalidInput, wantMsg: domain.RawInputSourceMessage,
		},
		{
			name: "raw_input too long: no transaction", req: with(valid, func(r *Request) { r.Source, r.RawInput = ptr("text"), ptr(strings.Repeat("x", 1001)) }),
			wantKind: apperr.InvalidInput, wantMsg: domain.RawInputRuleMessage,
		},
		{
			name: "raw_input with a tab: no transaction", req: with(valid, func(r *Request) { r.Source, r.RawInput = ptr("text"), ptr("a\tb") }),
			wantKind: apperr.InvalidInput, wantMsg: domain.RawInputRuleMessage,
		},
	}
	// 17:30 UTC on 2 October 2026 is 3 October in Bangkok: the latest date is 2027-10-03.
	now := func() time.Time { return time.Date(2026, time.October, 2, 17, 30, 0, 0, time.UTC) }
	bangkok := time.FixedZone("ICT", 7*60*60)
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
			findN := 0
			p := New(utx, now, bangkok, Ports{
				Find: findFn(func(ctx context.Context, o, got uuid.UUID) (domain.Transaction, bool, error) {
					call(ctx, "find", o)
					if got != id || findN >= len(tt.finds) {
						t.Fatalf("find %s (call %d)", got, findN)
					}
					f := tt.finds[findN]
					findN++
					return f.t, f.found, nil
				}),
				Category: categoryFn(func(ctx context.Context, o, got uuid.UUID) (categoriesdomain.Category, bool, error) {
					call(ctx, "category", o)
					if tt.category == nil {
						return categoriesdomain.Category{}, false, nil
					}
					return *tt.category, true, nil
				}),
				Insert: insertFn(func(ctx context.Context, nt transactionsinsertport.NewTransaction) (domain.Transaction, bool, error) {
					call(ctx, "insert", nt.OwnerID)
					want := transactionsinsertport.NewTransaction{
						ID: id, OwnerID: user, Amount: 14500, Currency: "THB", OccurredOn: nt.OccurredOn, Merchant: "M",
						CategoryID: cat, Source: domain.SourceManual, RawInput: tt.wantRaw,
					}
					if tt.wantSource != "" {
						want.Source = tt.wantSource
					}
					if nt != want || nt.OccurredOn.String() != "2026-01-15" {
						t.Errorf("inserted %+v, want %+v", nt, want)
					}
					if !tt.inserted {
						return domain.Transaction{}, false, tt.insertErr
					}
					return domain.Transaction{ID: id, OwnerID: user, Amount: nt.Amount}, true, tt.insertErr
				}),
			})
			tt.req.UserID = user
			resp, err := p.Execute(t.Context(), tt.req)
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			var ae *apperr.Error
			if tt.wantMsg != "" && (!errors.As(err, &ae) || ae.Message != tt.wantMsg) {
				t.Errorf("error = %v, want message %q", err, tt.wantMsg)
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
			if err == nil && (resp.Created != tt.wantCreated || resp.Transaction.ID != id || resp.Transaction.OwnerID != user) {
				t.Errorf("response = %+v, want created %v", resp, tt.wantCreated)
			}
		})
	}
}

func with(r Request, change func(*Request)) Request {
	change(&r)
	return r
}
