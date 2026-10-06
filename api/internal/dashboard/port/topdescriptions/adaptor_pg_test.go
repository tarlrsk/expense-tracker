package topdescriptions

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

func TestTopNeedsTransaction(t *testing.T) {
	_, err := NewPG().Top(t.Context(), domain.OwnerScope(uuid.New()), domain.NewMonth(2026, time.March), domain.TopDescriptionsLimit)
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestTopRefusesBadArguments(t *testing.T) {
	scope := domain.OwnerScope(uuid.New())
	tests := []struct {
		name  string
		month domain.Month
		limit int
		want  error
	}{
		{"zero limit", domain.NewMonth(2026, time.March), 0, errNoLimit},
		{"negative limit", domain.NewMonth(2026, time.March), -1, errNoLimit},
		{"no month", domain.Month{}, domain.TopDescriptionsLimit, errNoMonth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewPG().Top(t.Context(), scope, tt.month, tt.limit); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

// summary writes rows as "description|total|count" lines.
func summary(rows []domain.DescriptionTotal) string {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%s|%s|%d", r.Description, r.Total, r.Count))
	}
	return strings.Join(lines, "\n")
}

// food is a spending row in Food on day of March 2026.
func food(day int, amount, merchant string) porttest.Tx {
	return porttest.Tx{Category: "Food", On: fmt.Sprintf("2026-03-%02d", day), Amount: amount, Merchant: merchant}
}

func TestTop(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()
	march := domain.NewMonth(2026, time.March)

	tests := []struct {
		name     string
		archived bool // add an archived expense category "Old trips"
		seed     []porttest.Tx
		month    domain.Month // March 2026 when zero
		limit    int          // domain.TopDescriptionsLimit when zero
		want     []string
	}{
		{
			name: "an empty month is empty",
		},
		{
			name: "upper and lower case and outer spaces are one row; inner differences are not",
			seed: []porttest.Tx{
				food(1, "10", "Grab"), food(2, "20", "Grab"), food(3, "30", "grab"), food(4, "40", " GRAB "),
				food(5, "50", "7-Eleven"), food(6, "60", "7 eleven"),
			},
			want: []string{"Grab|100.00|4", "7 eleven|60.00|1", "7-Eleven|50.00|1"},
		},
		{
			name: "the shown spelling is the most used one, trimmed",
			seed: []porttest.Tx{food(1, "1", " GRAB "), food(2, "1", "GRAB"), food(3, "1", "grab")},
			want: []string{"GRAB|3.00|3"},
		},
		{
			name: "a tie on use goes to the spelling used most recently",
			seed: []porttest.Tx{food(9, "5", "Makro"), food(1, "5", "MAKRO")},
			want: []string{"Makro|10.00|2"},
		},
		{
			name: "a full tie goes to the first spelling in byte order",
			seed: []porttest.Tx{food(3, "5", "Big C"), food(3, "5", "BIG C")},
			want: []string{"BIG C|10.00|2"},
		},
		{
			name: "only the month's own uses choose the spelling",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-02-10", Amount: "1", Merchant: "grab"},
				{Category: "Food", On: "2026-02-11", Amount: "1", Merchant: "grab"},
				{Category: "Food", On: "2026-02-12", Amount: "1", Merchant: "grab"},
				food(1, "2", "Grab"), food(2, "2", "Grab"), food(3, "2", "grab"),
			},
			want: []string{"Grab|6.00|3"},
		},
		{
			name: "empty and spaces-only descriptions are left out",
			seed: []porttest.Tx{food(1, "1000", ""), food(2, "2000", "   "), food(3, "3", "Cafe")},
			want: []string{"Cafe|3.00|1"},
		},
		{
			name: "only the first and last day of the month count, not the days around it",
			seed: []porttest.Tx{
				{Category: "Food", On: "2026-02-28", Amount: "1000", Merchant: "Grab"},
				food(1, "1", "Grab"), food(31, "2", "Grab"),
				{Category: "Food", On: "2026-04-01", Amount: "2000", Merchant: "Grab"},
				{Category: "Food", On: "2026-04-01", Amount: "3000", Merchant: "April only"},
			},
			want: []string{"Grab|3.00|2"},
		},
		{
			name: "December does not include the 1st of January",
			seed: []porttest.Tx{
				{Category: "Food", On: "2025-12-31", Amount: "1", Merchant: "Grab"},
				{Category: "Food", On: "2026-01-01", Amount: "1000", Merchant: "New year"},
			},
			month: domain.NewMonth(2025, time.December),
			want:  []string{"Grab|1.00|1"},
		},
		{
			name: "a leap-year February ends on the 29th",
			seed: []porttest.Tx{
				{Category: "Food", On: "2024-02-29", Amount: "1", Merchant: "Leap"},
				{Category: "Food", On: "2024-03-01", Amount: "1000", Merchant: "March"},
			},
			month: domain.NewMonth(2024, time.February),
			want:  []string{"Leap|1.00|1"},
		},
		{
			name:     "an archived category counts, income never does",
			archived: true,
			seed: []porttest.Tx{
				{Category: "Old trips", On: "2026-03-10", Amount: "250.25", Merchant: "Airline"},
				{Category: "Salary", On: "2026-03-25", Amount: "50000", Merchant: "Company"},
				{Category: "Other income", On: "2026-03-02", Amount: "300", Merchant: "Airline"},
				food(11, "49.75", "Cafe"),
			},
			want: []string{"Airline|250.25|1", "Cafe|49.75|1"},
		},
		{
			name: "at most the limit, largest first",
			seed: []porttest.Tx{
				food(1, "1", "A"), food(1, "2", "B"), food(1, "3", "C"), food(1, "4", "D"),
				food(1, "5", "E"), food(1, "6", "F"), food(1, "7", "G"),
			},
			want: []string{"G|7.00|1", "F|6.00|1", "E|5.00|1", "D|4.00|1", "C|3.00|1"},
		},
		{
			name:  "a smaller limit",
			seed:  []porttest.Tx{food(1, "1", "A"), food(1, "2", "B"), food(1, "3", "C")},
			limit: 2,
			want:  []string{"C|3.00|1", "B|2.00|1"},
		},
		{
			name: "a tie on the total goes to the count, then to the key in byte order",
			seed: []porttest.Tx{
				food(1, "100", "One payment"),
				food(1, "50", "Two payments"), food(2, "50", "Two payments"),
				food(1, "10", "zeta"), food(1, "10", "Alpha"), food(1, "10", "7-eleven"),
			},
			want: []string{"Two payments|100.00|2", "One payment|100.00|1", "7-eleven|10.00|1", "Alpha|10.00|1", "zeta|10.00|1"},
		},
		{
			name: "decimals and large sums are exact",
			seed: []porttest.Tx{
				food(1, "0.10", "Tea"), food(2, "0.20", "Tea"),
				food(1, "9999999999.99", "Big"), food(2, "9999999999.99", "Big"), food(3, "9999999999.99", "Big"),
			},
			want: []string{"Big|29999999999.97|3", "Tea|0.30|2"},
		},
		{
			name: "Thai descriptions",
			seed: []porttest.Tx{
				food(1, "45", "กาแฟ"), food(2, "50", "กาแฟ"), food(3, "55", " กาแฟ "), food(4, "60", "ข้าวมันไก่"),
			},
			want: []string{"กาแฟ|150.00|3", "ข้าวมันไก่|60.00|1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := porttest.NewUser(t)
			if tt.archived {
				porttest.NewCategory(t, user, "Old trips", "✈️", "expense", 20, true)
			}
			porttest.Seed(t, user, tt.seed...)
			month, limit := tt.month, tt.limit
			if month.IsZero() {
				month = march
			}
			if limit == 0 {
				limit = domain.TopDescriptionsLimit
			}
			got := porttest.Call(t, d, user, func(ctx context.Context) ([]domain.DescriptionTotal, error) {
				return p.Top(ctx, domain.OwnerScope(user), month, limit)
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

func TestTopCrossUser(t *testing.T) {
	d := porttest.OpenDB(t)
	p := NewPG()
	a, b := porttest.NewUser(t), porttest.NewUser(t)
	march := domain.NewMonth(2026, time.March)
	porttest.Seed(t, a, food(5, "100", "Grab"))
	porttest.Seed(t, b, food(5, "7777", "grab"), food(6, "7777", "grab"), food(7, "1", "B secret shop"))
	top := func(user, owner uuid.UUID) string {
		t.Helper()
		return summary(porttest.Call(t, d, user, func(ctx context.Context) ([]domain.DescriptionTotal, error) {
			return p.Top(ctx, domain.OwnerScope(owner), march, domain.TopDescriptionsLimit)
		}))
	}

	if s := top(a, a); s != "Grab|100.00|1" {
		t.Errorf("A's own top list:\n%s", s)
	}
	if s := top(b, b); s != "grab|15554.00|2\nB secret shop|1.00|1" {
		t.Errorf("B's own top list:\n%s", s)
	}
	if s := top(a, b); s != "" {
		t.Errorf("A asking with B's scope got\n%s", s)
	}
}
