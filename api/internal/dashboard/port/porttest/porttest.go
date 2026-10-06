// Package porttest has the helpers the dashboard port tests share: the API's database opened as
// app_login on the Docker test database, users and categories, transactions seeded as the test
// superuser, and a call inside the caller's user transaction. Only tests import it.
package porttest

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
)

// OpenDB opens the API's database as app_login on the test database.
func OpenDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(t.Context(), db.Config{URL: dbtest.LoginURL(t), StatementTimeout: 5 * time.Second, MaxOpenConns: 4})
	if err != nil {
		t.Fatalf("open as %s: %v", dbtest.LoginRole, err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// NewUser inserts an account as the test superuser (the users trigger seeds its default
// categories, such as Food, Transport, Shopping and Salary); it and everything cascading from it
// are removed when the test ends.
func NewUser(t *testing.T) uuid.UUID {
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

// Category returns the id of the user's category called name.
func Category(t *testing.T, user uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := dbtest.DB(t).QueryRowContext(t.Context(),
		"select id from categories where owner_id = $1 and name = $2", user, name).Scan(&id); err != nil {
		t.Fatalf("category %q: %v", name, err)
	}
	return id
}

// NewCategory inserts a category for the user and returns its id; kind is expense or income.
func NewCategory(t *testing.T, user uuid.UUID, name, icon, kind string, sortOrder int, archived bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := dbtest.DB(t).QueryRowContext(t.Context(),
		`insert into categories (owner_id, name, icon, kind, sort_order, archived) values ($1, $2, $3, $4, $5, $6)
		 returning id`, user, name, icon, kind, sortOrder, archived).Scan(&id); err != nil {
		t.Fatalf("insert category %q: %v", name, err)
	}
	return id
}

// Tx is one transaction to seed.
type Tx struct {
	// Category is the name of one of the user's categories.
	Category string
	// On is occurred_on, YYYY-MM-DD.
	On string
	// Amount is the amount as decimal text, such as "145.50".
	Amount string
	// Merchant is the description, stored exactly as given (outer spaces included).
	Merchant string
}

// Seed inserts the transactions for the user as the test superuser.
func Seed(t *testing.T, user uuid.UUID, txs ...Tx) {
	t.Helper()
	super := dbtest.DB(t)
	for _, x := range txs {
		res, err := super.ExecContext(t.Context(),
			`insert into transactions (owner_id, amount, occurred_on, merchant, category_id)
			 select $1, $2::numeric, $3::date, $4, c.id from categories c where c.owner_id = $1 and c.name = $5`,
			user, x.Amount, x.On, x.Merchant, x.Category)
		if err != nil {
			t.Fatalf("insert transaction %+v: %v", x, err)
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			t.Fatalf("insert transaction %+v: %d rows (%v); is the category name right?", x, n, err)
		}
	}
}

// Call runs fn inside a user transaction of user, as the API does, and returns its result; an
// error fails the test.
func Call[T any](t *testing.T, d *db.DB, user uuid.UUID, fn func(ctx context.Context) (T, error)) T {
	t.Helper()
	var got T
	err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
		var err error
		got, err = fn(ctx)
		return err
	})
	if err != nil {
		t.Fatalf("call as %s: %v", user, err)
	}
	return got
}
