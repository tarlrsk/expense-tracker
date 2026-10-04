package reserve

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Tests of the adaptor on the Docker test database, logged in as app_login like the API.

func TestReserveNeedsTransaction(t *testing.T) {
	_, _, err := NewPG().Reserve(t.Context(), uuid.New(), day(t, "2026-10-04"), 5)
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestReserve(t *testing.T) {
	d := openDB(t)
	p := NewPG()
	d1, d2 := day(t, "2026-10-04"), day(t, "2026-10-05")
	// reserve runs the port as user for owner and returns its answer.
	reserve := func(user, owner uuid.UUID, on transactionsdomain.Date, limit int) (int, bool, error) {
		t.Helper()
		var (
			count    int
			reserved bool
		)
		err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
			var err error
			count, reserved, err = p.Reserve(ctx, owner, on, limit)
			return err
		})
		return count, reserved, err
	}
	mustReserve := func(user uuid.UUID, on transactionsdomain.Date, limit int) (int, bool) {
		t.Helper()
		count, reserved, err := reserve(user, user, on, limit)
		if err != nil {
			t.Fatalf("reserve for %s on %s: %v", user, on, err)
		}
		return count, reserved
	}

	t.Run("counts up to the limit, then stops without writing", func(t *testing.T) {
		a := newUser(t)
		for want := 1; want <= 3; want++ {
			if count, ok := mustReserve(a, d1, 3); !ok || count != want {
				t.Fatalf("reserve %d = %d, %v; want %d, true", want, count, ok, want)
			}
		}
		for range 2 {
			if count, ok := mustReserve(a, d1, 3); ok || count != 0 {
				t.Errorf("reserve at the limit = %d, %v; want 0, false", count, ok)
			}
		}
		if got := usage(t, a, d1); got != 3 {
			t.Errorf("stored count %d, want 3", got)
		}
	})

	t.Run("a lower limit stops a count already above it", func(t *testing.T) {
		a := newUser(t)
		for range 3 {
			mustReserve(a, d1, 10)
		}
		if _, ok := mustReserve(a, d1, 2); ok {
			t.Error("reserved over a lower limit")
		}
		if got := usage(t, a, d1); got != 3 {
			t.Errorf("stored count %d, want 3", got)
		}
	})

	t.Run("each day has its own count", func(t *testing.T) {
		a := newUser(t)
		mustReserve(a, d1, 1)
		if _, ok := mustReserve(a, d1, 1); ok {
			t.Fatal("reserved over the limit")
		}
		if count, ok := mustReserve(a, d2, 1); !ok || count != 1 {
			t.Errorf("the next day = %d, %v; want 1, true", count, ok)
		}
		if usage(t, a, d1) != 1 || usage(t, a, d2) != 1 {
			t.Errorf("stored counts %d, %d; want 1, 1", usage(t, a, d1), usage(t, a, d2))
		}
	})

	t.Run("limit 0 never reserves and writes no row", func(t *testing.T) {
		a := newUser(t)
		for _, limit := range []int{0, -1} {
			if count, ok := mustReserve(a, d1, limit); ok || count != 0 {
				t.Errorf("limit %d = %d, %v; want 0, false", limit, count, ok)
			}
		}
		if n := rows(t, a); n != 0 {
			t.Errorf("%d rows, want none", n)
		}
	})

	t.Run("cross-user: B's count never affects A's, and A cannot count for B", func(t *testing.T) {
		a, b := newUser(t), newUser(t)
		mustReserve(b, d1, 1)
		if _, ok := mustReserve(b, d1, 1); ok {
			t.Fatal("B reserved over the limit")
		}
		if count, ok := mustReserve(a, d1, 1); !ok || count != 1 {
			t.Errorf("A after B is at the limit = %d, %v; want 1, true", count, ok)
		}
		// Row-level security refuses a row for anyone but the transaction user.
		if _, _, err := reserve(a, b, d1, 5); err == nil {
			t.Error("A counted a use for B")
		}
		if _, _, err := reserve(a, b, d2, 5); err == nil {
			t.Error("A inserted a row for B")
		}
		if usage(t, b, d1) != 1 || rows(t, b) != 1 || usage(t, a, d1) != 1 {
			t.Errorf("B %d (%d rows), A %d; want B 1 (1 row), A 1", usage(t, b, d1), rows(t, b), usage(t, a, d1))
		}
	})

	t.Run("concurrent reservations never pass the limit", func(t *testing.T) {
		a := newUser(t)
		const limit, tries = 5, 12
		var (
			wg  sync.WaitGroup
			mu  sync.Mutex
			got int
		)
		for range tries {
			wg.Go(func() {
				_, ok, err := reserve(a, a, d1, limit)
				if err != nil {
					t.Errorf("reserve: %v", err)
				}
				if ok {
					mu.Lock()
					got++
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		if got != limit || usage(t, a, d1) != limit {
			t.Errorf("%d reserved, stored %d; want %d", got, usage(t, a, d1), limit)
		}
	})
}

func day(t *testing.T, s string) transactionsdomain.Date {
	t.Helper()
	d, ok := transactionsdomain.ParseDate(s)
	if !ok {
		t.Fatalf("not a date: %s", s)
	}
	return d
}

// openDB opens the API's database as app_login on the test database.
func openDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(t.Context(), db.Config{URL: dbtest.LoginURL(t), StatementTimeout: 5 * time.Second, MaxOpenConns: 8})
	if err != nil {
		t.Fatalf("open as %s: %v", dbtest.LoginRole, err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// newUser inserts an account as the test superuser; it and its usage rows are removed when the
// test ends.
func newUser(t *testing.T) uuid.UUID {
	t.Helper()
	super := dbtest.DB(t)
	var id uuid.UUID
	if err := super.QueryRowContext(t.Context(), "insert into users (email) values ($1) returning id",
		"u-"+uuid.NewString()+"@example.test").Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := super.ExecContext(context.WithoutCancel(t.Context()), "delete from users where id = $1", id); err != nil {
			t.Errorf("remove user %s: %v", id, err)
		}
	})
	if _, err := super.ExecContext(t.Context(), "insert into profiles (id) values ($1)", id); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	return id
}

// usage is the user's stored parse count of the day, read as the superuser; 0 without a row.
func usage(t *testing.T, user uuid.UUID, on transactionsdomain.Date) int {
	t.Helper()
	var n int
	err := dbtest.DB(t).QueryRowContext(t.Context(),
		"select parse_count from ai_usage where owner_id = $1 and day = $2::date", user, on.String()).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// rows is how many usage rows the user has.
func rows(t *testing.T, user uuid.UUID) int {
	t.Helper()
	var n int
	if err := dbtest.DB(t).QueryRowContext(t.Context(), "select count(*) from ai_usage where owner_id = $1", user).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
