package categorybreakdown

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/porttest"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Tests of the adaptor on the Docker test database, logged in as app_login like the API.

func TestBreakdownNeedsTransaction(t *testing.T) {
	_, err := NewPG().Breakdown(t.Context(), domain.OwnerScope(uuid.New()), domain.NewMonth(2026, time.March))
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestBreakdownNeedsMonth(t *testing.T) {
	_, err := NewPG().Breakdown(t.Context(), domain.OwnerScope(uuid.New()), domain.Month{})
	if !errors.Is(err, errNoMonth) {
		t.Fatalf("error = %v, want errNoMonth", err)
	}
}

// summary writes rows as "name total count" lines, with "(archived)" after an archived name.
func summary(rows []domain.CategoryTotal) string {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		name := r.Name
		if r.Archived {
			name += " (archived)"
		}
		lines = append(lines, fmt.Sprintf("%s %s %d", name, r.Total, r.Count))
	}
	return strings.Join(lines, "\n")
}

func TestBreakdown(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()

	tests := []struct {
		name     string
		archived bool // add an archived expense category "Old trips"
		seed     []porttest.Tx
		month    domain.Month
		want     []string
	}{
		{
			name:  "an empty month is empty",
			month: domain.NewMonth(2026, time.March),
		},
		{
			name: "only the first and last day of the month count, not the days around it",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-01-31", Amount: "1"},
				{Category: "Food", On: "2026-02-01", Amount: "10"},
				{Category: "Food", On: "2026-02-28", Amount: "100"},
				{Category: "Food", On: "2026-03-01", Amount: "1000"},
				{Category: "Transport", On: "2026-03-01", Amount: "5"},
			},
			month: domain.NewMonth(2026, time.February),
			want:  []string{"Food 110.00 2"},
		},
		{
			name: "a leap-year February ends on the 29th",
			seed: []porttest.Tx{
				{Category: "Food", On: "2024-02-29", Amount: "100"},
				{Category: "Food", On: "2024-03-01", Amount: "1000"},
			},
			month: domain.NewMonth(2024, time.February),
			want:  []string{"Food 100.00 1"},
		},
		{
			name: "a 30-day month",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-04-30", Amount: "100"},
				{Category: "Food", On: "2026-05-01", Amount: "1000"},
			},
			month: domain.NewMonth(2026, time.April),
			want:  []string{"Food 100.00 1"},
		},
		{
			name: "December does not include the 1st of January",
			seed: []porttest.Tx{
				{Category: "Food", On: "2025-12-31", Amount: "100"},
				{Category: "Food", On: "2026-01-01", Amount: "1000"},
				{Category: "Shopping", On: "2026-01-01", Amount: "1000"},
			},
			month: domain.NewMonth(2025, time.December),
			want:  []string{"Food 100.00 1"},
		},
		{
			name: "income is never listed",
			seed: []porttest.Tx{
				{Category: "Salary", On: "2026-03-25", Amount: "50000"},
				{Category: "Other income", On: "2026-03-02", Amount: "10"},
				{Category: "Food", On: "2026-03-03", Amount: "145.50"},
			},
			month: domain.NewMonth(2026, time.March),
			want:  []string{"Food 145.50 1"},
		},
		{
			name: "only income gives an empty breakdown",
			seed: []porttest.Tx{
				{Category: "Salary", On: "2026-03-25", Amount: "50000"},
			},
			month: domain.NewMonth(2026, time.March),
		},
		{
			name:     "an archived category with spending is listed and flagged",
			archived: true,
			seed: []porttest.Tx{
				{Category: "Old trips", On: "2026-03-10", Amount: "250.25"},
				{Category: "Food", On: "2026-03-11", Amount: "49.75"},
			},
			month: domain.NewMonth(2026, time.March),
			want:  []string{"Old trips (archived) 250.25 1", "Food 49.75 1"},
		},
		{
			name: "decimals are exact and counts are per category",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-03-01", Amount: "0.10"},
				{Category: "Food", On: "2026-03-02", Amount: "0.20"},
				{Category: "Food", On: "2026-03-02", Amount: "0.05"},
				{Category: "Transport", On: "2026-03-02", Amount: "0.30"},
			},
			month: domain.NewMonth(2026, time.March),
			want:  []string{"Food 0.35 3", "Transport 0.30 1"},
		},
		{
			name: "a large sum is exact",
			seed: []porttest.Tx{
				{Category: "Shopping", On: "2026-03-01", Amount: "9999999999.99"},
				{Category: "Shopping", On: "2026-03-02", Amount: "9999999999.99"},
				{Category: "Shopping", On: "2026-03-03", Amount: "9999999999.99"},
			},
			month: domain.NewMonth(2026, time.March),
			want:  []string{"Shopping 29999999999.97 3"},
		},
		{
			// Default sort orders: Food 1, Transport 3, Shopping 5, Health 6.
			name: "largest first; a tie on the total falls back to the category order",
			seed: []porttest.Tx{
				{Category: "Health", On: "2026-03-01", Amount: "50"},
				{Category: "Shopping", On: "2026-03-01", Amount: "100"},
				{Category: "Food", On: "2026-03-02", Amount: "60"},
				{Category: "Food", On: "2026-03-03", Amount: "40"},
				{Category: "Transport", On: "2026-03-04", Amount: "300"},
			},
			month: domain.NewMonth(2026, time.March),
			want:  []string{"Transport 300.00 1", "Food 100.00 2", "Shopping 100.00 1", "Health 50.00 1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := porttest.NewUser(t)
			if tt.archived {
				porttest.NewCategory(t, user, "Old trips", "✈️", "expense", 20, true)
			}
			porttest.Seed(t, user, tt.seed...)
			got := porttest.Call(t, d, user, func(ctx context.Context) ([]domain.CategoryTotal, error) {
				return p.Breakdown(ctx, domain.OwnerScope(user), tt.month)
			})
			if got == nil {
				t.Fatal("got a nil slice, want an empty or filled one")
			}
			if s, want := summary(got), strings.Join(tt.want, "\n"); s != want {
				t.Errorf("got\n%s\nwant\n%s", s, want)
			}
		})
	}
}

func TestBreakdownRowFields(t *testing.T) {
	d := porttest.OpenDB(t)
	user := porttest.NewUser(t)
	old := porttest.NewCategory(t, user, "Old trips", "✈️", "expense", 20, true)
	food := porttest.Category(t, user, "Food")
	porttest.Seed(t, user,
		porttest.Tx{Category: "Old trips", On: "2026-03-10", Amount: "250.25"},
		porttest.Tx{Category: "Food", On: "2026-03-11", Amount: "49.75"},
	)
	got := porttest.Call(t, d, user, func(ctx context.Context) ([]domain.CategoryTotal, error) {
		return NewPG().Breakdown(ctx, domain.OwnerScope(user), domain.NewMonth(2026, time.March))
	})
	want := []domain.CategoryTotal{
		{CategoryID: old, Name: "Old trips", Icon: "✈️", Archived: true, Total: 25025, Count: 1},
		{CategoryID: food, Name: "Food", Icon: "🍜", Archived: false, Total: 4975, Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBreakdownTieOnSortOrderFallsBackToID(t *testing.T) {
	d := porttest.OpenDB(t)
	user := porttest.NewUser(t)
	x := porttest.NewCategory(t, user, "Tie X", "", "expense", 30, false)
	y := porttest.NewCategory(t, user, "Tie Y", "", "expense", 30, false)
	porttest.Seed(t, user,
		porttest.Tx{Category: "Tie Y", On: "2026-03-10", Amount: "10"},
		porttest.Tx{Category: "Tie X", On: "2026-03-10", Amount: "10"},
	)
	first, second := x, y
	if bytes.Compare(y[:], x[:]) < 0 {
		first, second = y, x
	}
	got := porttest.Call(t, d, user, func(ctx context.Context) ([]domain.CategoryTotal, error) {
		return NewPG().Breakdown(ctx, domain.OwnerScope(user), domain.NewMonth(2026, time.March))
	})
	if len(got) != 2 || got[0].CategoryID != first || got[1].CategoryID != second {
		t.Errorf("got %+v, want %s then %s", got, first, second)
	}
}

func TestBreakdownCrossUser(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()
	a, b := porttest.NewUser(t), porttest.NewUser(t)
	march := domain.NewMonth(2026, time.March)
	porttest.NewCategory(t, b, "B only", "", "expense", 20, false)
	porttest.Seed(t, a, porttest.Tx{Category: "Food", On: "2026-03-05", Amount: "100"})
	porttest.Seed(t, b,
		porttest.Tx{Category: "Food", On: "2026-03-05", Amount: "7777"},
		porttest.Tx{Category: "Transport", On: "2026-03-06", Amount: "1"},
		porttest.Tx{Category: "B only", On: "2026-03-07", Amount: "2"},
	)
	breakdown := func(user, owner uuid.UUID) []domain.CategoryTotal {
		t.Helper()
		return porttest.Call(t, d, user, func(ctx context.Context) ([]domain.CategoryTotal, error) {
			return p.Breakdown(ctx, domain.OwnerScope(owner), march)
		})
	}

	got := breakdown(a, a)
	if s := summary(got); s != "Food 100.00 1" {
		t.Errorf("A's own breakdown:\n%s", s)
	}
	if len(got) == 1 && got[0].CategoryID != porttest.Category(t, a, "Food") {
		t.Errorf("A's Food row has id %s, not A's category", got[0].CategoryID)
	}
	if s := summary(breakdown(b, b)); s != "Food 7777.00 1\nB only 2.00 1\nTransport 1.00 1" {
		t.Errorf("B's own breakdown:\n%s", s)
	}
	if got := breakdown(a, b); len(got) != 0 {
		t.Errorf("A asking with B's scope got\n%s", summary(got))
	}
}
