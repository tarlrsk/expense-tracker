package reserve

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	accountdomain "github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	categoriesdomain "github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	usageFn func(ctx context.Context, ownerID uuid.UUID, day transactionsdomain.Date, limit int) (int, bool, error)
	listFn  func(ctx context.Context, ownerID uuid.UUID) ([]categoriesdomain.Category, error)
	styleFn func(ctx context.Context, userID uuid.UUID) (accountdomain.DescriptionStyle, bool, error)
)

func (f usageFn) Reserve(ctx context.Context, o uuid.UUID, d transactionsdomain.Date, l int) (int, bool, error) {
	return f(ctx, o, d, l)
}

func (f listFn) List(ctx context.Context, o uuid.UUID) ([]categoriesdomain.Category, error) {
	return f(ctx, o)
}

func (f styleFn) Get(ctx context.Context, u uuid.UUID) (accountdomain.DescriptionStyle, bool, error) {
	return f(ctx, u)
}

func TestExecute(t *testing.T) {
	user := uuid.New()
	food, old, salary := uuid.New(), uuid.New(), uuid.New()
	all := []categoriesdomain.Category{
		{ID: food, Name: "Food", Kind: categoriesdomain.KindExpense},
		{ID: old, Name: "Old", Kind: categoriesdomain.KindExpense, Archived: true},
		{ID: salary, Name: "Salary", Kind: categoriesdomain.KindIncome},
	}
	boom := errors.New("boom")
	tests := []struct {
		name      string
		reserved  bool
		usageErr  error
		listErr   error
		style     accountdomain.DescriptionStyle // "" means no profile row
		styleErr  error
		wantErr   error
		wantCats  []uuid.UUID
		wantStyle accountdomain.DescriptionStyle
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "reserved: the active categories in the caller's order and the caller's style", reserved: true,
			style: accountdomain.DescriptionStyleCleanName, wantStyle: accountdomain.DescriptionStyleCleanName,
			wantCats: []uuid.UUID{food, salary}, wantCalls: []string{"reserve", "list", "style"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "reserved, no profile row: the default style", reserved: true, wantStyle: accountdomain.DescriptionStyleAsTyped,
			wantCats: []uuid.UUID{food, salary}, wantCalls: []string{"reserve", "list", "style"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "the style read fails: the reservation is rolled back", reserved: true, styleErr: boom, wantErr: boom,
			wantCalls: []string{"reserve", "list", "style"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{name: "at the limit: nothing read", wantCalls: []string{"reserve"}, wantTx: []txtest.Outcome{txtest.Committed}},
		{
			name: "the reservation fails", usageErr: boom, wantErr: boom,
			wantCalls: []string{"reserve"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "the list fails: the reservation is rolled back", reserved: true, listErr: boom, wantErr: boom,
			wantCalls: []string{"reserve", "list"}, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
	}
	// 17:30 UTC on 2 October 2026 is 3 October in Bangkok.
	now := func() time.Time { return time.Date(2026, time.October, 2, 17, 30, 0, 0, time.UTC) }
	bangkok := time.FixedZone("ICT", 7*60*60)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			var calls []string
			call := func(ctx context.Context, name string, o uuid.UUID) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user || o != user {
					t.Errorf("%s outside the caller's user transaction (%v) or for %s", name, err, o)
				}
				calls = append(calls, name)
			}
			p := New(utx, now, bangkok, 7, Ports{
				Usage: usageFn(func(ctx context.Context, o uuid.UUID, d transactionsdomain.Date, limit int) (int, bool, error) {
					call(ctx, "reserve", o)
					if d.String() != "2026-10-03" || limit != 7 {
						t.Errorf("reserve on %s with limit %d, want 2026-10-03 and 7", d, limit)
					}
					if tt.usageErr != nil || !tt.reserved {
						return 0, false, tt.usageErr
					}
					return 1, true, nil
				}),
				Categories: listFn(func(ctx context.Context, o uuid.UUID) ([]categoriesdomain.Category, error) {
					call(ctx, "list", o)
					return all, tt.listErr
				}),
				DescriptionStyle: styleFn(func(ctx context.Context, o uuid.UUID) (accountdomain.DescriptionStyle, bool, error) {
					call(ctx, "style", o)
					return tt.style, tt.style != "", tt.styleErr
				}),
			})
			resp, err := p.Execute(t.Context(), Request{UserID: user})
			if !errors.Is(err, tt.wantErr) || (err == nil) != (tt.wantErr == nil) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err == nil {
				var cats []uuid.UUID
				for _, c := range resp.Categories {
					cats = append(cats, c.ID)
				}
				if resp.Reserved != tt.reserved || !slices.Equal(cats, tt.wantCats) || resp.Day.String() != "2026-10-03" ||
					resp.DescriptionStyle != tt.wantStyle {
					t.Errorf("response = reserved %v, day %s, categories %v, style %q; want %v, 2026-10-03, %v, %q",
						resp.Reserved, resp.Day, cats, resp.DescriptionStyle, tt.reserved, tt.wantCats, tt.wantStyle)
				}
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", calls, tt.wantCalls)
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
