package list

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/porttest"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/export/domain"
	txdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Tests of the adaptor on the Docker test database, logged in as app_login like the API. The
// users, categories and calls come from the dashboard's port helpers.

func TestListNeedsTransaction(t *testing.T) {
	_, err := NewPG().List(t.Context(), domain.OwnerScope(uuid.New()), Filter{Limit: 10})
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestListRefusesNoLimit(t *testing.T) {
	for _, limit := range []int{0, -1} {
		if _, err := NewPG().List(t.Context(), domain.OwnerScope(uuid.New()), Filter{Limit: limit}); !errors.Is(err, errNoLimit) {
			t.Errorf("limit %d: error = %v, want errNoLimit", limit, err)
		}
	}
}

// seeded is one transaction to insert and the line it gives in summary.
type seeded struct {
	category, on, amount, merchant, note, source string
}

// seed inserts the transactions for user as the test superuser and returns them with their ids,
// in the export's order (date, then id).
func seed(t *testing.T, user uuid.UUID, txs ...seeded) []stored {
	t.Helper()
	out := make([]stored, 0, len(txs))
	for _, x := range txs {
		var id uuid.UUID
		if err := dbtest.DB(t).QueryRowContext(t.Context(),
			`insert into transactions (owner_id, amount, occurred_on, merchant, note, source, category_id)
			 select $1, $2::numeric, $3::date, $4, $5, $6, c.id from categories c where c.owner_id = $1 and c.name = $7
			 returning id`,
			user, x.amount, x.on, x.merchant, x.note, x.source, x.category).Scan(&id); err != nil {
			t.Fatalf("insert %+v: %v", x, err)
		}
		out = append(out, stored{id: id, seeded: x})
	}
	slices.SortFunc(out, func(a, b stored) int {
		if c := strings.Compare(a.on, b.on); c != 0 {
			return c
		}
		return bytes.Compare(a.id[:], b.id[:])
	})
	return out
}

type stored struct {
	id uuid.UUID
	seeded
}

// line writes what the export holds of a transaction.
func (s stored) line(kind string) string {
	return strings.Join([]string{s.on, s.amount, kind, s.category, s.merchant, s.note, s.source}, "|")
}

func summary(rows []domain.Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, strings.Join([]string{r.OccurredOn.String(), r.Amount, r.Kind, r.Category, r.Description, r.Note, r.Source}, "|"))
	}
	return out
}

func ids(rows []domain.Row) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func date(t *testing.T, s string) *txdomain.Date {
	t.Helper()
	d, ok := txdomain.ParseDate(s)
	if !ok {
		t.Fatalf("not a date: %q", s)
	}
	return &d
}

func list(t *testing.T, d *db.DB, caller uuid.UUID, scope domain.Scope, f Filter) []domain.Row {
	t.Helper()
	return porttest.Call(t, d, caller, func(ctx context.Context) ([]domain.Row, error) {
		return NewPG().List(ctx, scope, f)
	})
}

func TestList(t *testing.T) {
	d := porttest.OpenDB(t)
	user := porttest.NewUser(t)
	porttest.NewCategory(t, user, "Old trips", "", "expense", 99, true)
	txs := seed(t, user,
		seeded{"Food", "2026-10-01", "145.00", "ข้าวมันไก่", "", "text"},
		seeded{"Salary", "2026-10-05", "30000.00", "Company", "October, after tax", "manual"},
		seeded{"Old trips", "2026-09-30", "1234567.89", "=Flight", "line one\nline two", "manual"},
		seeded{"Food", "2026-10-01", "0.50", " Grab ", "", "manual"},
		seeded{"Food", "2026-10-01", "60.00", "", "", "manual"},
		seeded{"Transport", "2026-10-31", "40.00", "BTS", "", "manual"},
		seeded{"Transport", "2026-11-01", "40.00", "BTS", "", "manual"},
	)
	kinds := map[string]string{"Food": "expense", "Transport": "expense", "Old trips": "expense", "Salary": "income"}
	want := func(keep func(stored) bool) []string {
		out := []string{}
		for _, s := range txs {
			if keep(s) {
				out = append(out, s.line(kinds[s.category]))
			}
		}
		return out
	}
	scope := domain.OwnerScope(user)

	t.Run("every transaction, oldest first, with the category's name and kind; text as stored", func(t *testing.T) {
		got := list(t, d, user, scope, Filter{Limit: 100})
		if w := want(func(stored) bool { return true }); !slices.Equal(summary(got), w) {
			t.Errorf("rows =\n%s\nwant\n%s", strings.Join(summary(got), "\n"), strings.Join(w, "\n"))
		}
	})

	t.Run("the period's ends are included", func(t *testing.T) {
		got := list(t, d, user, scope, Filter{From: date(t, "2026-10-01"), To: date(t, "2026-10-31"), Limit: 100})
		w := want(func(s stored) bool { return s.on >= "2026-10-01" && s.on <= "2026-10-31" })
		if !slices.Equal(summary(got), w) {
			t.Errorf("rows =\n%s\nwant\n%s", strings.Join(summary(got), "\n"), strings.Join(w, "\n"))
		}
		if got := list(t, d, user, scope, Filter{From: date(t, "2026-10-31"), Limit: 100}); len(got) != 2 {
			t.Errorf("from 2026-10-31: %d rows, want 2", len(got))
		}
		if got := list(t, d, user, scope, Filter{To: date(t, "2026-09-30"), Limit: 100}); len(got) != 1 {
			t.Errorf("to 2026-09-30: %d rows, want 1", len(got))
		}
		if got := list(t, d, user, scope, Filter{From: date(t, "2027-01-01"), Limit: 100}); got == nil || len(got) != 0 {
			t.Errorf("an empty period: %#v, want an empty, non-nil slice", got)
		}
	})

	t.Run("pages after a cursor give every row once, in order, also inside one day", func(t *testing.T) {
		for _, size := range []int{1, 2, 3, 7} {
			var all []uuid.UUID
			var after *domain.Cursor
			for range len(txs) + 1 {
				page := list(t, d, user, scope, Filter{After: after, Limit: size})
				all = append(all, ids(page)...)
				if len(page) < size {
					break
				}
				c := domain.CursorAfter(page[len(page)-1])
				after = &c
			}
			wantIDs := make([]uuid.UUID, 0, len(txs))
			for _, s := range txs {
				wantIDs = append(wantIDs, s.id)
			}
			if !slices.Equal(all, wantIDs) {
				t.Errorf("page size %d: ids %v, want %v", size, all, wantIDs)
			}
		}
	})
}

// One user's export never holds another user's rows: the scope is the one ownership condition,
// and row-level security stops a scope that is not the caller's (ADR-0014).
func TestListCrossUser(t *testing.T) {
	d := porttest.OpenDB(t)
	a, b := porttest.NewUser(t), porttest.NewUser(t)
	seed(t, a, seeded{"Food", "2026-10-01", "145.00", "Grab", "", "manual"})
	seed(t, b,
		seeded{"Food", "2026-10-01", "99.99", "zqwx", "secret", "manual"},
		seeded{"Salary", "2026-10-02", "5.00", "Makro", "", "manual"},
	)
	tests := []struct {
		name   string
		caller uuid.UUID
		scope  uuid.UUID
		want   []string
	}{
		{"A's scope as A: only A's row", a, a, []string{"2026-10-01|145.00|expense|Food|Grab||manual"}},
		{"B's scope as B: only B's rows", b, b, []string{
			"2026-10-01|99.99|expense|Food|zqwx|secret|manual", "2026-10-02|5.00|income|Salary|Makro||manual",
		}},
		{"B's scope as A: nothing (row-level security)", a, b, []string{}},
		{"A's scope as B: nothing (row-level security)", b, a, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summary(list(t, d, tt.caller, domain.OwnerScope(tt.scope), Filter{Limit: 100}))
			if !slices.Equal(got, tt.want) {
				t.Errorf("rows = %v, want %v", got, tt.want)
			}
		})
	}
}
