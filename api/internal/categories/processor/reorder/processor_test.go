package reorder

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	lockFn     func(ctx context.Context, ownerID uuid.UUID) error
	listFn     func(ctx context.Context, ownerID uuid.UUID) ([]domain.Category, error)
	setOrderFn func(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) error
)

func (f lockFn) Lock(ctx context.Context, o uuid.UUID) error { return f(ctx, o) }
func (f listFn) List(ctx context.Context, o uuid.UUID) ([]domain.Category, error) {
	return f(ctx, o)
}
func (f setOrderFn) SetOrder(ctx context.Context, o uuid.UUID, ids []uuid.UUID) error {
	return f(ctx, o, ids)
}

func TestExecute(t *testing.T) {
	user := uuid.New()
	a, b, c, archived, stranger := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	current := []domain.Category{
		{ID: a, SortOrder: 1}, {ID: archived, SortOrder: 2, Archived: true}, {ID: b, SortOrder: 3}, {ID: c, SortOrder: 4},
	}
	tests := []struct {
		name      string
		ids       []uuid.UUID
		current   []domain.Category
		wantKind  apperr.Kind
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "every active category once", ids: []uuid.UUID{c, a, b}, current: current,
			wantCalls: []string{"lock", "list", "set", "list"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "no active category: an empty list", ids: []uuid.UUID{}, current: []domain.Category{{ID: archived, Archived: true}},
			wantCalls: []string{"lock", "list", "set", "list"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{name: "no ids: no transaction", ids: nil, wantKind: apperr.InvalidInput},
		{name: "a repeated id: no transaction", ids: []uuid.UUID{a, b, c, a}, wantKind: apperr.InvalidInput},
		{
			name: "one missing", ids: []uuid.UUID{a, b}, current: current,
			wantKind: apperr.Conflict, wantCalls: []string{"lock", "list"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "an archived one", ids: []uuid.UUID{a, b, c, archived}, current: current,
			wantKind: apperr.Conflict, wantCalls: []string{"lock", "list"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "an archived one instead of an active one", ids: []uuid.UUID{a, b, archived}, current: current,
			wantKind: apperr.Conflict, wantCalls: []string{"lock", "list"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "an unknown one", ids: []uuid.UUID{a, b, c, stranger}, current: current,
			wantKind: apperr.Conflict, wantCalls: []string{"lock", "list"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "an unknown one instead of an active one", ids: []uuid.UUID{a, b, stranger}, current: current,
			wantKind: apperr.Conflict, wantCalls: []string{"lock", "list"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			var calls []string
			// Every port runs inside the one user transaction for the caller, the lock first.
			call := func(ctx context.Context, name string, owner uuid.UUID) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user {
					t.Errorf("%s outside the caller's user transaction: %v", name, err)
				}
				if owner != user {
					t.Errorf("%s for %s, want the caller", name, owner)
				}
				calls = append(calls, name)
			}
			var written []uuid.UUID
			p := New(utx, Ports{
				Lock: lockFn(func(ctx context.Context, o uuid.UUID) error { call(ctx, "lock", o); return nil }),
				List: listFn(func(ctx context.Context, o uuid.UUID) ([]domain.Category, error) {
					call(ctx, "list", o)
					return tt.current, nil
				}),
				SetOrder: setOrderFn(func(ctx context.Context, o uuid.UUID, ids []uuid.UUID) error {
					call(ctx, "set", o)
					written = ids
					return nil
				}),
			})
			resp, err := p.Execute(t.Context(), Request{UserID: user, IDs: tt.ids})
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
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
			if err != nil {
				return
			}
			if !slices.Equal(written, tt.ids) {
				t.Errorf("written order = %v, want %v", written, tt.ids)
			}
			if len(resp.Categories) != len(tt.current) {
				t.Errorf("response has %d categories, want the whole list (%d)", len(resp.Categories), len(tt.current))
			}
		})
	}
}
