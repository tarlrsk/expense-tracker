package find

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Tests of the adaptor on the Docker test database, logged in as app_login like the API.

func TestFindNeedsTransaction(t *testing.T) {
	_, _, err := NewPG().Find(t.Context(), uuid.New(), "grab")
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestFind(t *testing.T) {
	d := openDB(t)
	a, b := newUser(t), newUser(t)
	aFood, bFood := category(t, a), category(t, b)
	aRule := insertRule(t, a, "7eleven", "7-Eleven", aFood)
	bRule := insertRule(t, b, "7eleven", "7 eleven", bFood)
	p := NewPG()
	find := func(user, owner uuid.UUID, key string) (domain.Rule, bool) {
		t.Helper()
		var (
			got   domain.Rule
			found bool
		)
		err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
			var err error
			got, found, err = p.Find(ctx, owner, key)
			return err
		})
		if err != nil {
			t.Fatalf("find %q for %s: %v", key, owner, err)
		}
		return got, found
	}

	t.Run("the caller's rule", func(t *testing.T) {
		got, found := find(a, a, "7eleven")
		if !found || got.ID != aRule || got.MerchantKey != "7eleven" || got.Merchant != "7-Eleven" || got.CategoryID != aFood ||
			got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
			t.Errorf("found %v: %+v", found, got)
		}
	})

	t.Run("an unknown key is not found", func(t *testing.T) {
		if got, found := find(a, a, "grab"); found {
			t.Errorf("found %+v", got)
		}
	})

	t.Run("each user gets their own rule for the same key", func(t *testing.T) {
		got, found := find(b, b, "7eleven")
		if !found || got.ID != bRule || got.CategoryID != bFood || got.Merchant != "7 eleven" {
			t.Errorf("B found %v: %+v", found, got)
		}
	})

	t.Run("another user's rule is not found, even when asked for by owner", func(t *testing.T) {
		c := newUser(t)
		if got, found := find(c, c, "7eleven"); found {
			t.Errorf("C found %+v", got)
		}
		if got, found := find(c, a, "7eleven"); found {
			t.Errorf("C asking for A's rule found %+v", got)
		}
	})

	t.Run("a rule whose category is archived is ignored, and found again once it is active", func(t *testing.T) {
		setArchived(t, aFood, true)
		if got, found := find(a, a, "7eleven"); found {
			t.Errorf("found %+v with its category archived", got)
		}
		if n := countRules(t, a); n != 1 {
			t.Errorf("A has %d rules, want the rule kept", n)
		}
		if _, found := find(b, b, "7eleven"); !found {
			t.Error("A's archived category hid B's rule")
		}
		setArchived(t, aFood, false)
		if got, found := find(a, a, "7eleven"); !found || got.ID != aRule {
			t.Errorf("after un-archiving: found %v, %+v", found, got)
		}
	})
}

// openDB opens the API's database as app_login on the test database.
func openDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(t.Context(), db.Config{URL: dbtest.LoginURL(t), StatementTimeout: 5 * time.Second, MaxOpenConns: 4})
	if err != nil {
		t.Fatalf("open as %s: %v", dbtest.LoginRole, err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// newUser inserts an account as the test superuser (the users trigger seeds its default
// categories); it and everything cascading from it are removed when the test ends.
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

// category returns the id of the user's first default category.
func category(t *testing.T, user uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := dbtest.DB(t).QueryRowContext(t.Context(),
		"select id from categories where owner_id = $1 order by sort_order limit 1", user).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// insertRule inserts a rule as the superuser and returns its id.
func insertRule(t *testing.T, user uuid.UUID, key, merchant string, category uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := dbtest.DB(t).QueryRowContext(t.Context(),
		"insert into merchant_rules (owner_id, merchant_key, merchant, category_id) values ($1, $2, $3, $4) returning id",
		user, key, merchant, category).Scan(&id); err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	return id
}

func setArchived(t *testing.T, category uuid.UUID, archived bool) {
	t.Helper()
	if _, err := dbtest.DB(t).ExecContext(t.Context(), "update categories set archived = $2 where id = $1", category, archived); err != nil {
		t.Fatalf("archive %s: %v", category, err)
	}
}

func countRules(t *testing.T, user uuid.UUID) int {
	t.Helper()
	var n int
	if err := dbtest.DB(t).QueryRowContext(t.Context(), "select count(*) from merchant_rules where owner_id = $1", user).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
