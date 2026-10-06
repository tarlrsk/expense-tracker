package overview

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	_ "time/tzdata" // the zone database, as in the API binary

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	totalsFn    func(ctx context.Context, scope domain.Scope, month domain.Month) (domain.MonthTotals, error)
	breakdownFn func(ctx context.Context, scope domain.Scope, month domain.Month) ([]domain.CategoryTotal, error)
	trendFn     func(ctx context.Context, scope domain.Scope, last domain.Month, months int) ([]domain.MonthSpending, error)
	topFn       func(ctx context.Context, scope domain.Scope, month domain.Month, limit int) ([]domain.DescriptionTotal, error)
)

func (f totalsFn) Totals(ctx context.Context, s domain.Scope, m domain.Month) (domain.MonthTotals, error) {
	return f(ctx, s, m)
}

func (f breakdownFn) Breakdown(ctx context.Context, s domain.Scope, m domain.Month) ([]domain.CategoryTotal, error) {
	return f(ctx, s, m)
}

func (f trendFn) Trend(ctx context.Context, s domain.Scope, m domain.Month, n int) ([]domain.MonthSpending, error) {
	return f(ctx, s, m, n)
}

func (f topFn) Top(ctx context.Context, s domain.Scope, m domain.Month, n int) ([]domain.DescriptionTotal, error) {
	return f(ctx, s, m, n)
}

func TestExecute(t *testing.T) {
	bangkok, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		t.Fatalf("load Asia/Bangkok: %v", err)
	}
	// 18:00 UTC on 31 October is already 1 November in Bangkok (UTC+7).
	clock := time.Date(2026, time.October, 31, 18, 0, 0, 0, time.UTC)
	user := uuid.New()
	boom := errors.New("boom")
	s := func(v string) *string { return &v }

	totals := domain.MonthTotals{Spent: 1245000, Income: 3000000}
	categories := []domain.CategoryTotal{{CategoryID: uuid.New(), Name: "Food", Icon: "🍜", Total: 420000, Count: 31}}
	top := []domain.DescriptionTotal{{Description: "Grab", Total: 145000, Count: 9}}
	trendOf := func(last domain.Month) []domain.MonthSpending {
		out := make([]domain.MonthSpending, domain.TrendMonths)
		for i := range out {
			out[i] = domain.MonthSpending{Month: last.AddMonths(i - domain.TrendMonths + 1)}
		}
		return out
	}
	allCalls := []string{"totals", "breakdown", "trend", "top"}

	tests := []struct {
		name      string
		zone      *time.Location
		month     *string
		failAt    string // the port that fails with boom
		wantMonth string
		wantKind  apperr.Kind
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "no month: the current month in the app time zone", zone: bangkok, wantMonth: "2026-11",
			wantCalls: allCalls, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "no month, UTC: the month before", zone: time.UTC, wantMonth: "2026-10",
			wantCalls: allCalls, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "a sent month", zone: bangkok, month: s("2026-02"), wantMonth: "2026-02",
			wantCalls: allCalls, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "one year after the current month", zone: bangkok, month: s("2027-11"), wantMonth: "2027-11",
			wantCalls: allCalls, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{name: "malformed month: no transaction, no query", zone: bangkok, month: s("2026-13"), wantKind: apperr.InvalidInput},
		{name: "empty month: no transaction, no query", zone: bangkok, month: s(""), wantKind: apperr.InvalidInput},
		{name: "beyond one year: no transaction, no query", zone: bangkok, month: s("2027-12"), wantKind: apperr.InvalidInput},
		{name: "before 2000: no transaction, no query", zone: bangkok, month: s("1999-12"), wantKind: apperr.InvalidInput},
		{
			name: "totals fail", zone: bangkok, month: s("2026-02"), wantMonth: "2026-02", failAt: "totals", wantKind: apperr.Internal,
			wantCalls: allCalls[:1], wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "breakdown fails", zone: bangkok, month: s("2026-02"), wantMonth: "2026-02", failAt: "breakdown", wantKind: apperr.Internal,
			wantCalls: allCalls[:2], wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "trend fails", zone: bangkok, month: s("2026-02"), wantMonth: "2026-02", failAt: "trend", wantKind: apperr.Internal,
			wantCalls: allCalls[:3], wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "top fails", zone: bangkok, month: s("2026-02"), wantMonth: "2026-02", failAt: "top", wantKind: apperr.Internal,
			wantCalls: allCalls, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			var calls []string
			// call checks one port call: inside the caller's user transaction, for the caller's
			// scope and the month asked; it returns boom when this port is the one to fail.
			call := func(ctx context.Context, name string, scope domain.Scope, month domain.Month) error {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user {
					t.Errorf("%s outside the caller's user transaction: %v", name, err)
				}
				if scope != domain.OwnerScope(user) {
					t.Errorf("%s for scope %+v, want the caller's own", name, scope)
				}
				if month.String() != tt.wantMonth {
					t.Errorf("%s for month %s, want %s", name, month, tt.wantMonth)
				}
				calls = append(calls, name)
				if name == tt.failAt {
					return boom
				}
				return nil
			}
			p := New(utx, func() time.Time { return clock }, tt.zone, Ports{
				Totals: totalsFn(func(ctx context.Context, sc domain.Scope, m domain.Month) (domain.MonthTotals, error) {
					return totals, call(ctx, "totals", sc, m)
				}),
				Breakdown: breakdownFn(func(ctx context.Context, sc domain.Scope, m domain.Month) ([]domain.CategoryTotal, error) {
					return categories, call(ctx, "breakdown", sc, m)
				}),
				Trend: trendFn(func(ctx context.Context, sc domain.Scope, m domain.Month, n int) ([]domain.MonthSpending, error) {
					if n != domain.TrendMonths {
						t.Errorf("trend of %d months, want %d", n, domain.TrendMonths)
					}
					return trendOf(m), call(ctx, "trend", sc, m)
				}),
				Top: topFn(func(ctx context.Context, sc domain.Scope, m domain.Month, n int) ([]domain.DescriptionTotal, error) {
					if n != domain.TopDescriptionsLimit {
						t.Errorf("top of %d rows, want %d", n, domain.TopDescriptionsLimit)
					}
					return top, call(ctx, "top", sc, m)
				}),
			})
			resp, err := p.Execute(t.Context(), Request{UserID: user, Month: tt.month})
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if tt.failAt != "" && !errors.Is(err, boom) {
				t.Errorf("error = %v, want it to wrap the port's error", err)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
			var outcomes []txtest.Outcome
			for _, r := range utx.Records() {
				if r.Role != tx.RoleUser || r.UserID != user {
					t.Errorf("transaction %+v, want a user transaction for the caller", r)
				}
				outcomes = append(outcomes, r.Outcome)
			}
			if !slices.Equal(outcomes, tt.wantTx) {
				t.Errorf("transactions = %v, want %v", outcomes, tt.wantTx)
			}
			if err != nil {
				return
			}
			want := Response{
				Month: resp.Month, Totals: totals, Categories: categories,
				Trend: trendOf(resp.Month), TopDescriptions: top,
			}
			if resp.Month.String() != tt.wantMonth || !reflect.DeepEqual(resp, want) {
				t.Errorf("response = %+v, want month %s and %+v", resp, tt.wantMonth, want)
			}
		})
	}
}

func TestExecuteWithoutCaller(t *testing.T) {
	utx := txtest.New()
	p := New(utx, time.Now, time.UTC, Ports{})
	_, err := p.Execute(t.Context(), Request{})
	if !errors.Is(err, errNotReadable) || apperr.KindOf(err) != apperr.Internal {
		t.Errorf("error = %v, want the fault errNotReadable", err)
	}
	if n := len(utx.Records()); n != 0 {
		t.Errorf("%d transactions opened, want none", n)
	}
}
