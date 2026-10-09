package transactionscsv

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/export/domain"
	exportlistport "github.com/tarlrsk/expense-tracker/api/internal/export/port/list"
	txdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type listFn func(ctx context.Context, scope domain.Scope, f exportlistport.Filter) ([]domain.Row, error)

func (f listFn) List(ctx context.Context, s domain.Scope, fl exportlistport.Filter) ([]domain.Row, error) {
	return f(ctx, s, fl)
}

func ptr(s string) *string { return &s }

// history is n rows, one a day from 1 October 2026, oldest first.
func history(t *testing.T, n int) []domain.Row {
	t.Helper()
	first, _ := txdomain.ParseDate("2026-10-01")
	rows := make([]domain.Row, 0, n)
	for i := range n {
		rows = append(rows, domain.Row{
			ID: uuid.New(), OccurredOn: first.AddDays(i), Amount: fmt.Sprintf("%d.00", i+1),
			Kind: "expense", Category: "Food", Description: fmt.Sprintf("row %d", i+1), Source: "manual",
		})
	}
	return rows
}

func TestExecutePages(t *testing.T) {
	user := uuid.New()
	const pageSize = 2
	for _, n := range []int{0, 1, 2, 3, 4, 5} {
		t.Run(fmt.Sprintf("%d rows", n), func(t *testing.T) {
			rows := history(t, n)
			utx := txtest.New()
			var afters []string
			p := New(utx, pageSize, Ports{List: listFn(func(ctx context.Context, s domain.Scope, f exportlistport.Filter) ([]domain.Row, error) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user {
					t.Errorf("list outside the caller's user transaction: %v", err)
				}
				if s.OwnerID != user {
					t.Errorf("scope %s, want the caller", s.OwnerID)
				}
				if f.Limit != pageSize {
					t.Errorf("limit %d, want %d", f.Limit, pageSize)
				}
				start := 0
				if f.After != nil {
					afters = append(afters, f.After.OccurredOn.String())
					start = slices.IndexFunc(rows, func(r domain.Row) bool { return r.ID == f.After.ID }) + 1
				}
				return rows[start:min(start+f.Limit, len(rows))], nil
			})})
			resp, err := p.Execute(t.Context(), Request{UserID: user})
			if err != nil {
				t.Fatal(err)
			}
			// A full page asks for the next one; the first short page ends the file.
			wantTx := n/pageSize + 1
			if got := len(utx.Records()); got != wantTx {
				t.Errorf("%d user transactions, want %d (one per page)", got, wantTx)
			}
			for _, r := range utx.Records() {
				if r.Outcome != txtest.Committed || r.UserID != user {
					t.Errorf("transaction %+v, want committed for the caller", r)
				}
			}
			var wantAfters []string
			for i := pageSize; i <= n; i += pageSize {
				wantAfters = append(wantAfters, rows[i-1].OccurredOn.String())
			}
			if !slices.Equal(afters, wantAfters) {
				t.Errorf("cursors %v, want %v", afters, wantAfters)
			}
			lines := strings.Split(strings.TrimSuffix(string(resp.Body), "\r\n"), "\r\n")
			if len(lines) != n+1 || lines[0] != "\ufeffDate,Amount,Kind,Category,Description,Note,Source" {
				t.Fatalf("file = %q, want the header and %d rows", resp.Body, n)
			}
			for i, r := range rows {
				if want := fmt.Sprintf("%s,%s,expense,Food,row %d,,manual", r.OccurredOn, r.Amount, i+1); lines[i+1] != want {
					t.Errorf("line %d = %q, want %q", i+1, lines[i+1], want)
				}
			}
			if resp.FileName != "satang-transactions.csv" {
				t.Errorf("file name %q", resp.FileName)
			}
		})
	}
}

func TestExecutePeriod(t *testing.T) {
	user := uuid.New()
	var got exportlistport.Filter
	p := New(txtest.New(), domain.PageSize, Ports{List: listFn(func(_ context.Context, _ domain.Scope, f exportlistport.Filter) ([]domain.Row, error) {
		got = f
		return []domain.Row{}, nil
	})})
	resp, err := p.Execute(t.Context(), Request{UserID: user, From: ptr("2026-10-01"), To: ptr("2026-10-31")})
	if err != nil {
		t.Fatal(err)
	}
	if got.From == nil || got.From.String() != "2026-10-01" || got.To == nil || got.To.String() != "2026-10-31" || got.After != nil || got.Limit != domain.PageSize {
		t.Errorf("filter = %+v", got)
	}
	if resp.FileName != "satang-transactions-2026-10-01-to-2026-10-31.csv" {
		t.Errorf("file name %q", resp.FileName)
	}
}

func TestExecuteBadPeriodOpensNoTransaction(t *testing.T) {
	utx := txtest.New()
	p := New(utx, domain.PageSize, Ports{List: listFn(func(context.Context, domain.Scope, exportlistport.Filter) ([]domain.Row, error) {
		t.Error("list called")
		return nil, nil
	})})
	for _, req := range []Request{
		{UserID: uuid.New(), From: ptr("2026-10-02"), To: ptr("2026-10-01")},
		{UserID: uuid.New(), From: ptr("2026-10")},
		{UserID: uuid.New(), To: ptr("")},
	} {
		if _, err := p.Execute(t.Context(), req); apperr.KindOf(err) != apperr.InvalidInput {
			t.Errorf("%+v: error = %v, want invalid_input", req, err)
		}
	}
	if n := len(utx.Records()); n != 0 {
		t.Errorf("%d transactions opened", n)
	}
}

func TestExecuteFailedPageFailsTheFile(t *testing.T) {
	user := uuid.New()
	rows := history(t, 3)
	boom := errors.New("boom")
	calls := 0
	p := New(txtest.New(), 2, Ports{List: listFn(func(context.Context, domain.Scope, exportlistport.Filter) ([]domain.Row, error) {
		calls++
		if calls == 2 {
			return nil, boom
		}
		return rows[:2], nil
	})})
	resp, err := p.Execute(t.Context(), Request{UserID: user})
	if !errors.Is(err, boom) || resp.Body != nil {
		t.Errorf("error = %v, body %q; want the page's error and no file", err, resp.Body)
	}
}
