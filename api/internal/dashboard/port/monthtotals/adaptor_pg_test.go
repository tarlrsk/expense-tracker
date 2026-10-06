package monthtotals

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/porttest"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Tests of the adaptor on the Docker test database, logged in as app_login like the API.

func TestTotalsNeedsTransaction(t *testing.T) {
	_, err := NewPG().Totals(t.Context(), domain.OwnerScope(uuid.New()), domain.NewMonth(2026, time.March))
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestTotalsNeedsMonth(t *testing.T) {
	_, err := NewPG().Totals(t.Context(), domain.OwnerScope(uuid.New()), domain.Month{})
	if !errors.Is(err, errNoMonth) {
		t.Fatalf("error = %v, want errNoMonth", err)
	}
}

func TestTotals(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()

	tests := []struct {
		name       string
		archived   bool // add an archived expense category "Old trips"
		seed       []porttest.Tx
		month      domain.Month
		spent      string
		income     string
		difference string
	}{
		{
			name:  "an empty month is zeros",
			month: domain.NewMonth(2026, time.March),
			spent: "0.00", income: "0.00", difference: "0.00",
		},
		{
			name: "only the first and last day of the month count, not the days around it",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-01-31", Amount: "1"},
				{Category: "Food", On: "2026-02-01", Amount: "10"},
				{Category: "Food", On: "2026-02-28", Amount: "100"},
				{Category: "Food", On: "2026-03-01", Amount: "1000"},
				{Category: "Salary", On: "2026-01-31", Amount: "2"},
				{Category: "Salary", On: "2026-02-28", Amount: "20"},
				{Category: "Salary", On: "2026-03-01", Amount: "200"},
			},
			month: domain.NewMonth(2026, time.February),
			spent: "110.00", income: "20.00", difference: "-90.00",
		},
		{
			name: "a leap-year February ends on the 29th",
			seed: []porttest.Tx{
				{Category: "Food", On: "2024-01-31", Amount: "1"},
				{Category: "Food", On: "2024-02-01", Amount: "10"},
				{Category: "Food", On: "2024-02-29", Amount: "100"},
				{Category: "Food", On: "2024-03-01", Amount: "1000"},
			},
			month: domain.NewMonth(2024, time.February),
			spent: "110.00", income: "0.00", difference: "-110.00",
		},
		{
			name: "a 30-day month",
			seed: []porttest.Tx{
				{Category: "Transport", On: "2026-03-31", Amount: "1"},
				{Category: "Transport", On: "2026-04-01", Amount: "10"},
				{Category: "Transport", On: "2026-04-30", Amount: "100"},
				{Category: "Transport", On: "2026-05-01", Amount: "1000"},
			},
			month: domain.NewMonth(2026, time.April),
			spent: "110.00", income: "0.00", difference: "-110.00",
		},
		{
			name: "December does not include the 1st of January",
			seed: []porttest.Tx{
				{Category: "Food", On: "2025-11-30", Amount: "1"},
				{Category: "Food", On: "2025-12-01", Amount: "10"},
				{Category: "Food", On: "2025-12-31", Amount: "100"},
				{Category: "Food", On: "2026-01-01", Amount: "1000"},
				{Category: "Salary", On: "2026-01-01", Amount: "5000"},
			},
			month: domain.NewMonth(2025, time.December),
			spent: "110.00", income: "0.00", difference: "-110.00",
		},
		{
			name:     "an archived category counts like any other",
			archived: true,
			seed: []porttest.Tx{
				{Category: "Old trips", On: "2026-03-10", Amount: "250.25"},
				{Category: "Food", On: "2026-03-11", Amount: "49.75"},
			},
			month: domain.NewMonth(2026, time.March),
			spent: "300.00", income: "0.00", difference: "-300.00",
		},
		{
			name: "income counts only as income",
			seed: []porttest.Tx{
				{Category: "Salary", On: "2026-03-25", Amount: "50000"},
				{Category: "Other income", On: "2026-03-02", Amount: "500.50"},
				{Category: "Food", On: "2026-03-03", Amount: "145.50"},
			},
			month: domain.NewMonth(2026, time.March),
			spent: "145.50", income: "50500.50", difference: "50355.00",
		},
		{
			name: "decimals are exact",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-03-01", Amount: "0.10"},
				{Category: "Food", On: "2026-03-02", Amount: "0.20"},
				{Category: "Salary", On: "2026-03-03", Amount: "0.01"},
			},
			month: domain.NewMonth(2026, time.March),
			spent: "0.30", income: "0.01", difference: "-0.29",
		},
		{
			name: "a large sum is exact",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-03-01", Amount: "9999999999.99"},
				{Category: "Food", On: "2026-03-02", Amount: "9999999999.99"},
				{Category: "Shopping", On: "2026-03-03", Amount: "9999999999.99"},
				{Category: "Shopping", On: "2026-03-31", Amount: "9999999999.99"},
				{Category: "Salary", On: "2026-03-15", Amount: "9999999999.99"},
			},
			month: domain.NewMonth(2026, time.March),
			spent: "39999999999.96", income: "9999999999.99", difference: "-29999999999.97",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := porttest.NewUser(t)
			if tt.archived {
				porttest.NewCategory(t, user, "Old trips", "✈️", "expense", 20, true)
			}
			porttest.Seed(t, user, tt.seed...)
			got := porttest.Call(t, d, user, func(ctx context.Context) (domain.MonthTotals, error) {
				return p.Totals(ctx, domain.OwnerScope(user), tt.month)
			})
			if got.Spent.String() != tt.spent || got.Income.String() != tt.income || got.Difference().String() != tt.difference {
				t.Errorf("spent %s, income %s, difference %s; want %s, %s, %s",
					got.Spent, got.Income, got.Difference(), tt.spent, tt.income, tt.difference)
			}
		})
	}
}

func TestTotalsCrossUser(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()
	a, b := porttest.NewUser(t), porttest.NewUser(t)
	march := domain.NewMonth(2026, time.March)
	porttest.Seed(t, a,
		porttest.Tx{Category: "Food", On: "2026-03-05", Amount: "100"},
		porttest.Tx{Category: "Salary", On: "2026-03-25", Amount: "1000"},
	)
	porttest.Seed(t, b,
		porttest.Tx{Category: "Food", On: "2026-03-05", Amount: "7777"},
		porttest.Tx{Category: "Salary", On: "2026-03-25", Amount: "88888"},
	)
	totals := func(user, owner uuid.UUID) domain.MonthTotals {
		t.Helper()
		return porttest.Call(t, d, user, func(ctx context.Context) (domain.MonthTotals, error) {
			return p.Totals(ctx, domain.OwnerScope(owner), march)
		})
	}

	if got := totals(a, a); got.Spent.String() != "100.00" || got.Income.String() != "1000.00" {
		t.Errorf("A's own totals: %s / %s, want 100.00 / 1000.00", got.Spent, got.Income)
	}
	if got := totals(b, b); got.Spent.String() != "7777.00" || got.Income.String() != "88888.00" {
		t.Errorf("B's own totals: %s / %s, want 7777.00 / 88888.00", got.Spent, got.Income)
	}
	if got := totals(a, b); got != (domain.MonthTotals{}) {
		t.Errorf("A asking with B's scope got %s / %s, want zeros", got.Spent, got.Income)
	}
}
