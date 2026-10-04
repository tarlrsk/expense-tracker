package update

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categoriesdomain "github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionsupdateport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/update"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	lockFn     func(ctx context.Context, ownerID, id uuid.UUID) (domain.Transaction, bool, error)
	updateFn   func(ctx context.Context, ownerID, id uuid.UUID, ch transactionsupdateport.Changes) (domain.Transaction, bool, error)
	categoryFn func(ctx context.Context, ownerID, id uuid.UUID) (categoriesdomain.Category, bool, error)
)

func (f lockFn) Lock(ctx context.Context, o, id uuid.UUID) (domain.Transaction, bool, error) {
	return f(ctx, o, id)
}

func (f updateFn) Update(ctx context.Context, o, id uuid.UUID, ch transactionsupdateport.Changes) (domain.Transaction, bool, error) {
	return f(ctx, o, id, ch)
}

func (f categoryFn) Find(ctx context.Context, o, id uuid.UUID) (categoriesdomain.Category, bool, error) {
	return f(ctx, o, id)
}

func TestExecute(t *testing.T) {
	user, other := uuid.New(), uuid.New()
	id, archivedCat, activeCat := uuid.New(), uuid.New(), uuid.New()
	date, _ := domain.ParseDate("2026-01-15")
	cur := domain.Transaction{
		ID: id, OwnerID: user, Amount: 14500, Currency: "THB", OccurredOn: date, Merchant: "Shop", CategoryID: archivedCat, Note: "n",
	}
	s := func(v string) *string { return &v }
	tests := []struct {
		name       string
		req        Request
		locked     *domain.Transaction // nil: not found
		wantKind   apperr.Kind
		wantCalls  []string
		wantChange string
		// the change flags of the response
		wantCategoryChanged, wantMerchantChanged bool
	}{
		{name: "no field", req: Request{}, wantKind: apperr.InvalidInput},
		{name: "bad amount", req: Request{Amount: s("0")}, wantKind: apperr.InvalidInput},
		{
			name: "same values change nothing", req: Request{Amount: s("145.0"), Merchant: s(" Shop "), CategoryID: s(archivedCat.String()), Currency: s("THB")},
			locked: &cur, wantCalls: []string{"lock"},
		},
		{
			name: "amount on an archived category", req: Request{Amount: s("20"), CategoryID: s(archivedCat.String())},
			locked: &cur, wantCalls: []string{"lock", "update"}, wantChange: "amount",
		},
		{
			name: "to an active category", req: Request{CategoryID: s(activeCat.String())},
			locked: &cur, wantCalls: []string{"lock", "category", "update"}, wantChange: "category", wantCategoryChanged: true,
		},
		{
			name: "merchant only", req: Request{Merchant: s(" Other "), Amount: s("145")},
			locked: &cur, wantCalls: []string{"lock", "update"}, wantChange: "merchant", wantMerchantChanged: true,
		},
		{
			name: "merchant cleared", req: Request{Merchant: s("")},
			locked: &cur, wantCalls: []string{"lock", "update"}, wantChange: "merchant", wantMerchantChanged: true,
		},
		{
			name: "to another archived category", req: Request{CategoryID: s(uuid.NewString())},
			locked: &cur, wantKind: apperr.InvalidInput, wantCalls: []string{"lock", "category"},
		},
		{name: "not found", req: Request{Amount: s("1")}, wantKind: apperr.NotFound, wantCalls: []string{"lock"}},
		{
			name: "a row authz does not let the caller change is not found", req: Request{Amount: s("1")},
			locked: &domain.Transaction{ID: id, OwnerID: other}, wantKind: apperr.NotFound, wantCalls: []string{"lock"},
		},
	}
	now := func() time.Time { return time.Date(2026, time.October, 3, 0, 0, 0, 0, time.UTC) }
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			p := New(txtest.New(), now, time.UTC, Ports{
				Lock: lockFn(func(_ context.Context, o, got uuid.UUID) (domain.Transaction, bool, error) {
					calls = append(calls, "lock")
					if tt.locked == nil {
						return domain.Transaction{}, false, nil
					}
					return *tt.locked, true, nil
				}),
				Category: categoryFn(func(_ context.Context, _, got uuid.UUID) (categoriesdomain.Category, bool, error) {
					calls = append(calls, "category")
					switch got {
					case activeCat:
						return categoriesdomain.Category{ID: got}, true, nil
					default:
						return categoriesdomain.Category{ID: got, Archived: true}, true, nil
					}
				}),
				Update: updateFn(func(_ context.Context, _, _ uuid.UUID, ch transactionsupdateport.Changes) (domain.Transaction, bool, error) {
					calls = append(calls, "update")
					var got string
					switch {
					case ch.Amount != nil && ch.CategoryID == nil && ch.Merchant == nil && ch.Currency == nil:
						got = "amount"
					case ch.CategoryID != nil && ch.Amount == nil:
						got = "category"
					case ch.Merchant != nil && ch.Amount == nil && ch.CategoryID == nil:
						got = "merchant"
					}
					if got != tt.wantChange {
						t.Errorf("changes = %+v, want only %s", ch, tt.wantChange)
					}
					return cur, true, nil
				}),
			})
			tt.req.UserID, tt.req.ID = user, id
			resp, err := p.Execute(t.Context(), tt.req)
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if resp.CategoryChanged != tt.wantCategoryChanged || resp.MerchantChanged != tt.wantMerchantChanged {
				t.Errorf("category changed %v, merchant changed %v; want %v, %v",
					resp.CategoryChanged, resp.MerchantChanged, tt.wantCategoryChanged, tt.wantMerchantChanged)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
		})
	}
}
