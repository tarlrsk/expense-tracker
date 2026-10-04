package upsert

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Tests of the adaptor on the Docker test database, logged in as app_login like the API.

func TestUpsertNeedsTransaction(t *testing.T) {
	_, err := NewPG().Upsert(t.Context(), NewRule{OwnerID: uuid.New(), MerchantKey: "grab", Merchant: "Grab", CategoryID: uuid.New()})
	if !errors.Is(err, tx.ErrNoTx) {
		t.Fatalf("error = %v, want tx.ErrNoTx", err)
	}
}

func TestUpsert(t *testing.T) {
	d := openDB(t)
	a, b := newUser(t), newUser(t)
	aFood, aTransport := categories(t, a)
	bFood, _ := categories(t, b)
	p := NewPG()
	upsert := func(user uuid.UUID, r NewRule) (domain.Rule, error) {
		t.Helper()
		var got domain.Rule
		err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
			var err error
			got, err = p.Upsert(ctx, r)
			return err
		})
		return got, err
	}

	first, err := upsert(a, NewRule{OwnerID: a, MerchantKey: "7eleven", Merchant: "7-Eleven", CategoryID: aFood})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if first.ID == uuid.Nil || first.MerchantKey != "7eleven" || first.Merchant != "7-Eleven" || first.CategoryID != aFood ||
		first.CreatedAt.IsZero() || !first.UpdatedAt.Equal(first.CreatedAt) {
		t.Errorf("inserted %+v", first)
	}

	t.Run("the latest choice wins: one row, merchant and category replaced", func(t *testing.T) {
		time.Sleep(time.Millisecond) // updated_at must move past created_at
		got, err := upsert(a, NewRule{OwnerID: a, MerchantKey: "7eleven", Merchant: "7 ELEVEN", CategoryID: aTransport})
		if err != nil {
			t.Fatalf("replace: %v", err)
		}
		if got.ID != first.ID || got.MerchantKey != "7eleven" || got.Merchant != "7 ELEVEN" || got.CategoryID != aTransport ||
			!got.CreatedAt.Equal(first.CreatedAt) || !got.UpdatedAt.After(first.UpdatedAt) {
			t.Errorf("replaced %+v, first %+v", got, first)
		}
		var (
			n        int
			merchant string
			category uuid.UUID
		)
		if err := dbtest.DB(t).QueryRowContext(t.Context(),
			"select count(*), min(merchant), min(category_id::text)::uuid from merchant_rules where owner_id = $1 and merchant_key = '7eleven'",
			a).Scan(&n, &merchant, &category); err != nil {
			t.Fatal(err)
		}
		if n != 1 || merchant != "7 ELEVEN" || category != aTransport {
			t.Errorf("stored: %d rows, %q, %s; want 1 row, \"7 ELEVEN\", %s", n, merchant, category, aTransport)
		}
	})

	t.Run("another user holds the same key with another category; A's rule is untouched", func(t *testing.T) {
		before := rules(t, a)
		got, err := upsert(b, NewRule{OwnerID: b, MerchantKey: "7eleven", Merchant: "7-11", CategoryID: bFood})
		if err != nil {
			t.Fatalf("B's upsert: %v", err)
		}
		if got.ID == first.ID || got.CategoryID != bFood || got.Merchant != "7-11" {
			t.Errorf("B's rule %+v", got)
		}
		if after := rules(t, a); after != before {
			t.Errorf("A's rules changed: %s, then %s", before, after)
		}
		if n := rules(t, b); n != "7eleven 7-11 "+bFood.String() {
			t.Errorf("B's rules: %s", n)
		}
	})

	t.Run("a rule for another user is refused by row-level security", func(t *testing.T) {
		before := rules(t, b)
		_, err := upsert(a, NewRule{OwnerID: b, MerchantKey: "grab", Merchant: "Grab secret", CategoryID: bFood})
		if err == nil {
			t.Fatal("A wrote a rule for B")
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("the error quotes the merchant: %v", err)
		}
		if after := rules(t, b); after != before {
			t.Errorf("B's rules changed: %s, then %s", before, after)
		}
	})

	t.Run("another user's category is refused by the foreign key, without quoting values", func(t *testing.T) {
		before := rules(t, a)
		_, err := upsert(a, NewRule{OwnerID: a, MerchantKey: "grab", Merchant: "Grab secret", CategoryID: bFood})
		if !errors.Is(err, gorm.ErrForeignKeyViolated) {
			t.Fatalf("error = %v, want a foreign-key violation", err)
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), bFood.String()) {
			t.Errorf("the error quotes a value: %v", err)
		}
		if after := rules(t, a); after != before {
			t.Errorf("A's rules changed: %s, then %s", before, after)
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

// categories returns the ids of the user's first two default categories.
func categories(t *testing.T, user uuid.UUID) (first, second uuid.UUID) {
	t.Helper()
	rows, err := dbtest.DB(t).QueryContext(t.Context(),
		"select id from categories where owner_id = $1 order by sort_order limit 2", user)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil || len(ids) != 2 {
		t.Fatalf("categories of %s: %v, %v", user, ids, err)
	}
	return ids[0], ids[1]
}

// rules is every rule of the user as one text, read as the superuser.
func rules(t *testing.T, user uuid.UUID) string {
	t.Helper()
	var s string
	if err := dbtest.DB(t).QueryRowContext(t.Context(),
		`select coalesce(string_agg(merchant_key || ' ' || merchant || ' ' || category_id, '; ' order by merchant_key), '')
		 from merchant_rules where owner_id = $1`, user).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}
