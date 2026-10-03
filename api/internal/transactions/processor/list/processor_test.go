package list

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionslistport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/list"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type listFn func(ctx context.Context, ownerID uuid.UUID, f transactionslistport.Filter) ([]domain.Transaction, error)

func (f listFn) List(ctx context.Context, o uuid.UUID, fl transactionslistport.Filter) ([]domain.Transaction, error) {
	return f(ctx, o, fl)
}

func TestExecute(t *testing.T) {
	user := uuid.New()
	d, _ := domain.ParseDate("2026-02-14")
	rows := func(n int, owner uuid.UUID) []domain.Transaction {
		out := make([]domain.Transaction, n)
		for i := range out {
			out[i] = domain.Transaction{ID: uuid.New(), OwnerID: owner, OccurredOn: d}
		}
		return out
	}
	s := func(v string) *string { return &v }
	tests := []struct {
		name      string
		req       Request
		rows      []domain.Transaction
		wantLimit int // the limit asked of the port
		wantLen   int
		wantNext  bool
		wantKind  apperr.Kind
	}{
		{name: "default page, more rows exist", rows: rows(51, user), wantLimit: 51, wantLen: 50, wantNext: true},
		{name: "default page, exactly 50", rows: rows(50, user), wantLimit: 51, wantLen: 50},
		{name: "limit 2 of 3", req: Request{Limit: s("2")}, rows: rows(3, user), wantLimit: 3, wantLen: 2, wantNext: true},
		{name: "empty", req: Request{Limit: s("200")}, wantLimit: 201},
		{name: "a row of another user is a fault, never shown", rows: rows(1, uuid.New()), wantLimit: 51, wantKind: apperr.Internal},
		{name: "bad limit", req: Request{Limit: s("0")}, wantKind: apperr.InvalidInput},
		{name: "bad cursor", req: Request{Cursor: s("x")}, wantKind: apperr.InvalidInput},
		{name: "month with from", req: Request{Month: s("2026-02"), From: s("2026-02-01")}, wantKind: apperr.InvalidInput},
		{name: "bad source", req: Request{Source: s("x")}, wantKind: apperr.InvalidInput},
		{name: "bad category", req: Request{Category: s("x")}, wantKind: apperr.InvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asked := 0
			p := New(txtest.New(), Ports{List: listFn(func(_ context.Context, o uuid.UUID, f transactionslistport.Filter) ([]domain.Transaction, error) {
				if o != user {
					t.Errorf("list for %s", o)
				}
				asked = f.Limit
				return tt.rows, nil
			})})
			tt.req.UserID = user
			resp, err := p.Execute(t.Context(), tt.req)
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if asked != tt.wantLimit {
				t.Errorf("asked the port for %d rows, want %d", asked, tt.wantLimit)
			}
			if err != nil {
				return
			}
			if len(resp.Transactions) != tt.wantLen || (resp.Next != nil) != tt.wantNext {
				t.Errorf("%d rows, next %v; want %d, %v", len(resp.Transactions), resp.Next, tt.wantLen, tt.wantNext)
			}
			if resp.Next != nil && *resp.Next != domain.CursorAfter(resp.Transactions[len(resp.Transactions)-1]) {
				t.Errorf("next = %+v, want after the last row of the page", resp.Next)
			}
		})
	}
}
