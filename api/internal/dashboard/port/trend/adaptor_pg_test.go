package trend

import (
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

func TestTrendNeedsTransaction(t *testing.T) {
	_, err := NewPG().Trend(t.Context(), domain.OwnerScope(uuid.New()), domain.NewMonth(2026, time.March), domain.TrendMonths)
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestTrendRefusesBadArguments(t *testing.T) {
	scope := domain.OwnerScope(uuid.New())
	tests := []struct {
		name   string
		last   domain.Month
		months int
		want   error
	}{
		{"zero months", domain.NewMonth(2026, time.March), 0, errNoMonths},
		{"negative months", domain.NewMonth(2026, time.March), -1, errNoMonths},
		{"no month", domain.Month{}, domain.TrendMonths, errNoMonth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewPG().Trend(t.Context(), scope, tt.last, tt.months); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

// summary writes rows as "YYYY-MM total" lines.
func summary(rows []domain.MonthSpending) string {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%s %s", r.Month, r.Spent))
	}
	return strings.Join(lines, "\n")
}

func TestTrend(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()

	tests := []struct {
		name     string
		archived bool // add an archived expense category "Old trips"
		seed     []porttest.Tx
		last     domain.Month
		months   int
		want     []string
	}{
		{
			name:   "no spending is six zero months",
			last:   domain.NewMonth(2026, time.March),
			months: domain.TrendMonths,
			want: []string{
				"2025-10 0.00", "2025-11 0.00", "2025-12 0.00", "2026-01 0.00", "2026-02 0.00", "2026-03 0.00",
			},
		},
		{
			name: "ending in March spans the year before; the days around the range do not count",
			seed: []porttest.Tx{
				{Category: "Food", On: "2025-09-30", Amount: "1000"},
				{Category: "Food", On: "2025-10-01", Amount: "1"},
				{Category: "Food", On: "2025-12-31", Amount: "2"},
				{Category: "Transport", On: "2025-12-15", Amount: "0.50"},
				{Category: "Food", On: "2026-01-01", Amount: "3"},
				{Category: "Food", On: "2026-02-28", Amount: "4"},
				{Category: "Food", On: "2026-03-01", Amount: "5"},
				{Category: "Food", On: "2026-03-31", Amount: "6"},
				{Category: "Food", On: "2026-04-01", Amount: "2000"},
			},
			last:   domain.NewMonth(2026, time.March),
			months: domain.TrendMonths,
			want: []string{
				"2025-10 1.00", "2025-11 0.00", "2025-12 2.50", "2026-01 3.00", "2026-02 4.00", "2026-03 11.00",
			},
		},
		{
			name: "ending in January spans the year before",
			seed: []porttest.Tx{
				{Category: "Food", On: "2025-07-31", Amount: "1000"},
				{Category: "Food", On: "2025-08-01", Amount: "1"},
				{Category: "Food", On: "2025-12-31", Amount: "2"},
				{Category: "Food", On: "2026-01-31", Amount: "3"},
				{Category: "Food", On: "2026-02-01", Amount: "2000"},
			},
			last:   domain.NewMonth(2026, time.January),
			months: domain.TrendMonths,
			want: []string{
				"2025-08 1.00", "2025-09 0.00", "2025-10 0.00", "2025-11 0.00", "2025-12 2.00", "2026-01 3.00",
			},
		},
		{
			name: "a leap-year February and a 30-day month keep their last day",
			seed: []porttest.Tx{
				{Category: "Food", On: "2024-02-29", Amount: "1"},
				{Category: "Food", On: "2024-03-01", Amount: "10"},
				{Category: "Food", On: "2024-04-30", Amount: "100"},
				{Category: "Food", On: "2024-05-01", Amount: "1000"},
			},
			last:   domain.NewMonth(2024, time.May),
			months: 4,
			want:   []string{"2024-02 1.00", "2024-03 10.00", "2024-04 100.00", "2024-05 1000.00"},
		},
		{
			name:     "archived categories count, income never does",
			archived: true,
			seed: []porttest.Tx{
				{Category: "Old trips", On: "2026-02-10", Amount: "250.25"},
				{Category: "Food", On: "2026-03-11", Amount: "49.75"},
				{Category: "Salary", On: "2026-03-25", Amount: "50000"},
				{Category: "Other income", On: "2026-01-02", Amount: "10"},
			},
			last:   domain.NewMonth(2026, time.March),
			months: 3,
			want:   []string{"2026-01 0.00", "2026-02 250.25", "2026-03 49.75"},
		},
		{
			name: "decimals and large sums are exact",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-02-01", Amount: "0.10"},
				{Category: "Food", On: "2026-02-02", Amount: "0.20"},
				{Category: "Shopping", On: "2026-03-01", Amount: "9999999999.99"},
				{Category: "Shopping", On: "2026-03-02", Amount: "9999999999.99"},
				{Category: "Shopping", On: "2026-03-03", Amount: "9999999999.99"},
			},
			last:   domain.NewMonth(2026, time.March),
			months: 2,
			want:   []string{"2026-02 0.30", "2026-03 29999999999.97"},
		},
		{
			name: "one month",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-02-28", Amount: "1"},
				{Category: "Food", On: "2026-03-15", Amount: "2"},
			},
			last:   domain.NewMonth(2026, time.March),
			months: 1,
			want:   []string{"2026-03 2.00"},
		},
		{
			name: "more than a year",
			seed: []porttest.Tx{
				{Category: "Food", On: "2024-12-31", Amount: "1"},
				{Category: "Food", On: "2026-01-01", Amount: "2"},
			},
			last:   domain.NewMonth(2026, time.January),
			months: 14,
			want: []string{
				"2024-12 1.00", "2025-01 0.00", "2025-02 0.00", "2025-03 0.00", "2025-04 0.00", "2025-05 0.00", "2025-06 0.00",
				"2025-07 0.00", "2025-08 0.00", "2025-09 0.00", "2025-10 0.00", "2025-11 0.00", "2025-12 0.00", "2026-01 2.00",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := porttest.NewUser(t)
			if tt.archived {
				porttest.NewCategory(t, user, "Old trips", "✈️", "expense", 20, true)
			}
			porttest.Seed(t, user, tt.seed...)
			got := porttest.Call(t, d, user, func(ctx context.Context) ([]domain.MonthSpending, error) {
				return p.Trend(ctx, domain.OwnerScope(user), tt.last, tt.months)
			})
			if s, want := summary(got), strings.Join(tt.want, "\n"); s != want {
				t.Errorf("got\n%s\nwant\n%s", s, want)
			}
			if len(got) > 0 && got[len(got)-1].Month != tt.last {
				t.Errorf("the last row is %s, want %s", got[len(got)-1].Month, tt.last)
			}
		})
	}
}

func TestTrendCrossUser(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()
	a, b := porttest.NewUser(t), porttest.NewUser(t)
	march := domain.NewMonth(2026, time.March)
	porttest.Seed(t, a,
		porttest.Tx{Category: "Food", On: "2026-02-05", Amount: "100"},
		porttest.Tx{Category: "Food", On: "2026-03-05", Amount: "200"},
	)
	porttest.Seed(t, b,
		porttest.Tx{Category: "Food", On: "2026-02-05", Amount: "7777"},
		porttest.Tx{Category: "Food", On: "2026-03-05", Amount: "8888"},
		porttest.Tx{Category: "Food", On: "2026-01-05", Amount: "9999"},
	)
	trend := func(user, owner uuid.UUID) string {
		t.Helper()
		return summary(porttest.Call(t, d, user, func(ctx context.Context) ([]domain.MonthSpending, error) {
			return p.Trend(ctx, domain.OwnerScope(owner), march, 3)
		}))
	}

	if s, want := trend(a, a), "2026-01 0.00\n2026-02 100.00\n2026-03 200.00"; s != want {
		t.Errorf("A's own trend:\n%s\nwant\n%s", s, want)
	}
	if s, want := trend(b, b), "2026-01 9999.00\n2026-02 7777.00\n2026-03 8888.00"; s != want {
		t.Errorf("B's own trend:\n%s\nwant\n%s", s, want)
	}
	if s, want := trend(a, b), "2026-01 0.00\n2026-02 0.00\n2026-03 0.00"; s != want {
		t.Errorf("A asking with B's scope got\n%s\nwant zeros", s)
	}
}
