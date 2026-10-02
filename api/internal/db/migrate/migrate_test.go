package migrate_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

// Up, down to nothing, and up again, in a throw-away database on the test server.
//
// The shared test database is migrated first, so it keeps using app_user and app_auth: the down
// sections must then leave both roles, and app_login with them, in place (they belong to the
// whole server), and tests in other packages running at the same time are not disturbed. The branch where down drops the
// roles is covered by TestNonSuperuserOwner.
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

	db := connect(t, cfg, throwAwayDB(t, shared, ""))
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

	checkNoLeftovers(t, db, "after down")
	if n := roleCount(t, shared, "app_user", "app_auth", "app_login"); n != 3 {
		t.Errorf("after down: %d of the 3 roles left; the shared test database still uses them", n)
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
