package create

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	categoriesinsertport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/insert"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	lockFn   func(ctx context.Context, ownerID uuid.UUID) error
	countFn  func(ctx context.Context, ownerID uuid.UUID) (int, error)
	insertFn func(ctx context.Context, c categoriesinsertport.NewCategory) (domain.Category, error)
)

func (f lockFn) Lock(ctx context.Context, o uuid.UUID) error          { return f(ctx, o) }
func (f countFn) Count(ctx context.Context, o uuid.UUID) (int, error) { return f(ctx, o) }
func (f insertFn) Insert(ctx context.Context, c categoriesinsertport.NewCategory) (domain.Category, error) {
	return f(ctx, c)
}

func TestExecute(t *testing.T) {
	user := uuid.New()
	tests := []struct {
		name      string
		req       Request
		count     int
		insertErr error
		want      categoriesinsertport.NewCategory
		wantKind  apperr.Kind
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "created, input normalised", req: Request{Name: "  Pet \t food ", Kind: "expense", Icon: " 🐶 "}, count: 13,
			want:      categoriesinsertport.NewCategory{OwnerID: user, Name: "Pet food", Icon: "🐶", Kind: domain.KindExpense},
			wantCalls: []string{"lock", "count", "insert"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "income without an icon", req: Request{Name: "Bonus", Kind: "income"}, count: 199,
			want:      categoriesinsertport.NewCategory{OwnerID: user, Name: "Bonus", Kind: domain.KindIncome},
			wantCalls: []string{"lock", "count", "insert"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{name: "empty name: no transaction", req: Request{Name: " ", Kind: "expense"}, wantKind: apperr.InvalidInput},
		{name: "long name: no transaction", req: Request{Name: strings.Repeat("x", 51), Kind: "expense"}, wantKind: apperr.InvalidInput},
		{name: "control character: no transaction", req: Request{Name: "a\x07b", Kind: "expense"}, wantKind: apperr.InvalidInput},
		{name: "bad kind: no transaction", req: Request{Name: "Pets", Kind: "transfer"}, wantKind: apperr.InvalidInput},
		{name: "no kind: no transaction", req: Request{Name: "Pets"}, wantKind: apperr.InvalidInput},
		{name: "long icon: no transaction", req: Request{Name: "Pets", Kind: "expense", Icon: strings.Repeat("i", 33)}, wantKind: apperr.InvalidInput},
		{
			name: "at the limit: conflict, nothing inserted", req: Request{Name: "Pets", Kind: "expense"}, count: domain.MaxCategories,
			wantKind: apperr.Conflict, wantCalls: []string{"lock", "count"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "name taken: conflict, rolled back", req: Request{Name: "Food", Kind: "expense"}, count: 13,
			insertErr: categoriesinsertport.ErrNameTaken,
			want:      categoriesinsertport.NewCategory{OwnerID: user, Name: "Food", Kind: domain.KindExpense},
			wantKind:  apperr.Conflict, wantCalls: []string{"lock", "count", "insert"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "database error: internal", req: Request{Name: "Food", Kind: "expense"}, count: 13, insertErr: errors.New("boom"),
			want:     categoriesinsertport.NewCategory{OwnerID: user, Name: "Food", Kind: domain.KindExpense},
			wantKind: apperr.Internal, wantCalls: []string{"lock", "count", "insert"}, wantTx: []txtest.Outcome{txtest.RolledBack},
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
			created := domain.Category{ID: uuid.New(), SortOrder: 14}
			p := New(utx, Ports{
				Lock:  lockFn(func(ctx context.Context, o uuid.UUID) error { call(ctx, "lock", o); return nil }),
				Count: countFn(func(ctx context.Context, o uuid.UUID) (int, error) { call(ctx, "count", o); return tt.count, nil }),
				Insert: insertFn(func(ctx context.Context, c categoriesinsertport.NewCategory) (domain.Category, error) {
					call(ctx, "insert", c.OwnerID)
					if c != tt.want {
						t.Errorf("inserted %+v, want %+v", c, tt.want)
					}
					return created, tt.insertErr
				}),
			})
			tt.req.UserID = user
			resp, err := p.Execute(t.Context(), tt.req)
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
			if err == nil && resp.Category != created {
				t.Errorf("category = %+v, want %+v", resp.Category, created)
			}
		})
	}
}
