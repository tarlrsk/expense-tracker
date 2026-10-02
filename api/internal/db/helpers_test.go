package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
)

// SQLSTATE codes the tests expect.
const (
	insufficientPrivilege = "42501"
	queryCanceled         = "57014"
	noActiveTransaction   = "25P01"
)

// testTimeout is the statement timeout of testDB unless a test sets another.
const testTimeout = 7 * time.Second

// testDB opens the API's database as app_login on the test database; change adjusts the config.
func testDB(t *testing.T, change ...func(*Config)) *DB {
	t.Helper()
	cfg := Config{StatementTimeout: testTimeout, MaxOpenConns: 4}
	for _, c := range change {
		c(&cfg)
	}
	d, err := open(t.Context(), dbtest.LoginConfig(t), cfg)
	if err != nil {
		t.Fatalf("open as %s: %v", dbtest.LoginRole, err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// newUser creates an account through WithAuthTx, as the account module will; the default
// categories come from the trigger. The account (and everything cascading from it) is removed
// by the test superuser when the test ends.
func newUser(t *testing.T, d *DB) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := d.WithAuthTx(t.Context(), func(ctx context.Context) error {
		c, err := AuthConn(ctx)
		if err != nil {
			return err
		}
		if err := c.Raw("insert into users (email) values (?) returning id", uuid.NewString()+"@example.test").Row().Scan(&id); err != nil {
			return err
		}
		return c.Exec("insert into profiles (id) values (?)", id).Error
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	shared := dbtest.DB(t)
	t.Cleanup(func() {
		if _, err := shared.ExecContext(context.WithoutCancel(t.Context()), "delete from users where id = $1", id); err != nil {
			t.Errorf("remove user %s: %v", id, err)
		}
	})
	return id
}

// categoryName reads the name of the user's category with sort_order 1 as the test superuser.
func categoryName(t *testing.T, user uuid.UUID) string {
	t.Helper()
	var name string
	if err := dbtest.DB(t).QueryRowContext(t.Context(),
		"select name from categories where owner_id = $1 and sort_order = 1", user).Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

// rename renames the user's first category inside the user transaction of ctx.
func rename(ctx context.Context, name string) error {
	c, err := UserConn(ctx)
	if err != nil {
		return err
	}
	return c.Exec("update categories set name = ? where sort_order = 1", name).Error
}

// sqlState returns the SQLSTATE of err, or "" when it is not a Postgres error.
func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func ident(name string) string { return pgx.Identifier{name}.Sanitize() }

// throwAwayRole creates a role that can log in with a random password, runs the extra
// statements (with %s replaced by the quoted role name) and returns a connection URL for it on
// database (the test database when empty). The role and its grants are removed when the test ends.
func throwAwayRole(t *testing.T, attributes, database string, extra ...string) (connURL, password string) {
	t.Helper()
	shared := dbtest.DB(t)
	role := "t" + randomHex(t, 6) + "_api"
	password = randomHex(t, 16)
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		for _, q := range []string{"drop owned by " + ident(role), "drop role " + ident(role)} {
			if _, err := shared.ExecContext(ctx, q); err != nil {
				t.Errorf("%s: %v", q, err)
			}
		}
	})
	if _, err := shared.ExecContext(t.Context(),
		"create role "+ident(role)+" login "+attributes+" password '"+password+"'"); err != nil {
		t.Fatalf("create role: %v", err)
	}
	for _, q := range extra {
		if _, err := shared.ExecContext(t.Context(), fmt.Sprintf(q, ident(role))); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	cfg := dbtest.Config(t)
	if database == "" {
		database = cfg.Database
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(role, password),
		Host:     cfg.Host + ":" + strconv.Itoa(int(cfg.Port)),
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	return u.String(), password
}

// throwAwayDB creates an empty database on the test server and drops it when the test ends.
func throwAwayDB(t *testing.T) string {
	t.Helper()
	shared := dbtest.DB(t)
	name := "expense_test_tmp_" + randomHex(t, 6)
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		if _, err := shared.ExecContext(ctx, "drop database if exists "+ident(name)+" with (force)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})
	if _, err := shared.ExecContext(t.Context(), "create database "+ident(name)+" template template0"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	return name
}

// roleOf returns the user name of a connection URL made by throwAwayRole.
func roleOf(t *testing.T, connURL string) string {
	t.Helper()
	u, err := url.Parse(connURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.User.Username()
}

// superuserOn connects to database as the test superuser; the pool is closed when the test ends.
func superuserOn(t *testing.T, database string) *sql.DB {
	t.Helper()
	cfg := dbtest.Config(t)
	cfg.Database = database
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// rootQuery runs a query on the pool outside any transaction, as the bare login role. Only tests
// may do this; the API cannot.
func rootQuery(t *testing.T, d *DB, q string, dest ...any) error {
	t.Helper()
	return d.root.WithContext(t.Context()).Raw(q).Row().Scan(dest...)
}
