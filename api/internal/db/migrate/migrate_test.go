package migrate_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

// Up, down to nothing, and up again, in a throw-away database on the test server.
//
// The shared test database is migrated first, so it keeps using app_user and app_auth: the down
// sections must then leave both roles in place (they belong to the whole server), and tests in
// other packages running at the same time are not disturbed.
func TestRoundTrip(t *testing.T) {
	shared := dbtest.DB(t)
	cfg := dbtest.Config(t)
	dir, err := dbtest.MigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migration files in %s: %v", dir, err)
	}

	db := throwAwayDB(t, shared, cfg)
	m, err := migrate.New(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()

	applied, err := m.Up(ctx)
	if err != nil {
		t.Fatalf("first up: %v", err)
	}
	if len(applied) != len(files) {
		t.Fatalf("first up applied %d migrations, want %d", len(applied), len(files))
	}
	if n := count(t, db, "select count(*) from pg_class where relnamespace = 'public'::regnamespace and relkind = 'r' and relname <> 'goose_db_version'"); n == 0 {
		t.Fatal("no tables after up")
	}

	for range files {
		if _, err := m.Down(ctx); err != nil {
			t.Fatalf("down: %v", err)
		}
	}
	if _, err := m.Down(ctx); !errors.Is(err, migrate.ErrNothingToRollBack) {
		t.Fatalf("down with nothing applied: %v, want ErrNothingToRollBack", err)
	}

	leftovers := []struct{ what, query string }{
		{"tables, views or sequences", `select count(*) from pg_class where relnamespace = 'public'::regnamespace
			and relkind in ('r', 'p', 'v', 'm', 'S', 'f') and relname not in ('goose_db_version', 'goose_db_version_id_seq')`},
		{"functions", "select count(*) from pg_proc where pronamespace = 'public'::regnamespace"},
		{"types", `select count(*) from pg_type where typnamespace = 'public'::regnamespace
			and typname not in ('goose_db_version', '_goose_db_version')`},
		{"app schema", "select count(*) from pg_namespace where nspname = 'app'"},
		{"extensions", "select count(*) from pg_extension where extname <> 'plpgsql'"},
	}
	for _, l := range leftovers {
		if n := count(t, db, l.query); n != 0 {
			t.Errorf("after down: %d %s left", n, l.what)
		}
	}
	if n := count(t, shared, "select count(*) from pg_roles where rolname in ('app_user', 'app_auth')"); n != 2 {
		t.Errorf("after down: %d of the 2 roles left; the shared test database still uses them", n)
	}

	applied, err = m.Up(ctx)
	if err != nil {
		t.Fatalf("second up: %v", err)
	}
	if len(applied) != len(files) {
		t.Errorf("second up applied %d migrations, want %d", len(applied), len(files))
	}
	status, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range status {
		if !s.Applied {
			t.Errorf("%s not applied after second up", s.Name)
		}
	}
}

// throwAwayDB creates an empty database on the test server, dropped when the test ends.
func throwAwayDB(t *testing.T, shared *sql.DB, cfg *pgx.ConnConfig) *sql.DB {
	t.Helper()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "expense_roundtrip_" + hex.EncodeToString(b)
	ident := pgx.Identifier{name}.Sanitize()

	// template0 never has connections, so CREATE DATABASE cannot clash with another session.
	if _, err := shared.ExecContext(t.Context(), "create database "+ident+" template template0"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	cfg.Database = name
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() {
		_ = db.Close()
		ctx := context.WithoutCancel(t.Context())
		if _, err := shared.ExecContext(ctx, "drop database "+ident+" with (force)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})
	return db
}

func count(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(t.Context(), q).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}
