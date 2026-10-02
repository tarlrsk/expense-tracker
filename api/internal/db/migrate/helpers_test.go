package migrate_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

// randomHex returns n random bytes as hex: unique names for throw-away objects.
func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func ident(name string) string { return pgx.Identifier{name}.Sanitize() }

// throwAwayDB creates an empty database on the test server, owned by owner ("" for the test
// superuser), and drops it when the test ends. It returns the database name.
// template0 never has connections, so CREATE DATABASE cannot clash with another session.
func throwAwayDB(t *testing.T, shared *sql.DB, owner string) string {
	t.Helper()
	name := "expense_test_tmp_" + randomHex(t, 6)
	q := "create database " + ident(name) + " template template0" //nolint:gosec // identifiers are quoted by pgx.Identifier
	if owner != "" {
		q += " owner " + ident(owner)
	}
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		if _, err := shared.ExecContext(ctx, "drop database if exists "+ident(name)+" with (force)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})
	if _, err := shared.ExecContext(t.Context(), q); err != nil {
		t.Fatalf("create database: %v", err)
	}
	return name
}

// throwAwayRoles drops the named roles, if they exist, when the test ends. Register it before
// creating them, and after throwAwayDB, so the databases that use them are dropped first.
func throwAwayRoles(t *testing.T, shared *sql.DB, roles ...string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		for _, r := range roles {
			if _, err := shared.ExecContext(ctx, "drop role if exists "+ident(r)); err != nil {
				t.Errorf("drop role %s: %v", r, err)
			}
		}
	})
}

// connect opens a database on the test server, as the test user unless cfg says otherwise.
func connect(t *testing.T, cfg *pgx.ConnConfig, database string) *sql.DB {
	t.Helper()
	c := cfg.Copy()
	c.Database = database
	db := stdlib.OpenDB(*c)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// testRoles are throw-away names for the three roles of the migrations.
type testRoles struct{ user, auth, login string }

func newTestRoles(t *testing.T) testRoles {
	t.Helper()
	p := "t" + randomHex(t, 6)
	return testRoles{user: p + "_user", auth: p + "_auth", login: p + "_login"}
}

func (r testRoles) all() []string { return []string{r.user, r.auth, r.login} }

// renamedMigrations copies the migration files into a temporary directory with the roles
// app_user, app_auth and app_login renamed, so a test can create and drop its own roles without
// touching the server-wide ones other tests use.
func renamedMigrations(t *testing.T, roles testRoles) string {
	t.Helper()
	src, err := dbtest.MigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(src, "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migration files in %s: %v", src, err)
	}
	dst := t.TempDir()
	replacer := strings.NewReplacer("app_user", roles.user, "app_auth", roles.auth, "app_login", roles.login)
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // path comes from the repository's migrations directory
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dst, filepath.Base(f))
		if err := os.WriteFile(out, []byte(replacer.Replace(string(b))), 0o600); err != nil { //nolint:gosec // out is in t.TempDir()
			t.Fatal(err)
		}
	}
	return dst
}

// downAll rolls back every applied migration, one file at a time, until none is left.
func downAll(t *testing.T, m *migrate.Migrator) {
	t.Helper()
	for {
		_, err := m.Down(t.Context())
		if errors.Is(err, migrate.ErrNothingToRollBack) {
			return
		}
		if err != nil {
			t.Fatalf("down: %v", err)
		}
	}
}

func count(t *testing.T, q queryer, query string, args ...any) int {
	t.Helper()
	var n int
	if err := q.QueryRowContext(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// leftovers lists what the migrations may have left in a database after a full down or a failed
// up: anything but goose's version table and plpgsql.
var leftovers = []struct{ what, query string }{
	{"tables, views or sequences", `select count(*) from pg_class where relnamespace = 'public'::regnamespace
		and relkind in ('r', 'p', 'v', 'm', 'S', 'f') and relname not in ('goose_db_version', 'goose_db_version_id_seq')`},
	{"functions", "select count(*) from pg_proc where pronamespace = 'public'::regnamespace"},
	{"types", `select count(*) from pg_type where typnamespace = 'public'::regnamespace
		and typname not in ('goose_db_version', '_goose_db_version')`},
	{"app schema", "select count(*) from pg_namespace where nspname = 'app'"},
	{"extensions", "select count(*) from pg_extension where extname <> 'plpgsql'"},
	{"default privileges", "select count(*) from pg_default_acl"},
}

func checkNoLeftovers(t *testing.T, db *sql.DB, when string) {
	t.Helper()
	for _, l := range leftovers {
		if n := count(t, db, l.query); n != 0 {
			t.Errorf("%s: %d %s left", when, n, l.what)
		}
	}
}

func roleCount(t *testing.T, shared *sql.DB, roles ...string) int {
	t.Helper()
	return count(t, shared, "select count(*) from pg_roles where rolname = any($1)", roles)
}
