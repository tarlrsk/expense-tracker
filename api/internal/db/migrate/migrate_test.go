package migrate_test

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

// migrationName is the file name form goose reads: a version number, an underscore, a name.
var migrationName = regexp.MustCompile(`^([0-9]+)_[a-z0-9_]+\.sql$`)

// The migration files are numbered 1..N without gaps or duplicates, and each has an Up and a
// Down section, in that order. Needs no database.
func TestMigrationFiles(t *testing.T) {
	dir, err := dbtest.MigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var versions []int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".sql" {
			continue
		}
		m := migrationName.FindStringSubmatch(name)
		if m == nil {
			t.Errorf("%s: name is not NNNN_name.sql", name)
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			t.Errorf("%s: version: %v", name, err)
			continue
		}
		versions = append(versions, v)

		b, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // path comes from the repository's migrations directory
		if err != nil {
			t.Fatal(err)
		}
		var ups, downs, upLine, downLine int
		sc := bufio.NewScanner(bytes.NewReader(b))
		for line := 1; sc.Scan(); line++ {
			switch string(bytes.TrimSpace(sc.Bytes())) {
			case "-- +goose Up":
				ups++
				upLine = line
			case "-- +goose Down":
				downs++
				downLine = line
			}
		}
		if err := sc.Err(); err != nil {
			t.Fatal(err)
		}
		if ups != 1 || downs != 1 {
			t.Errorf("%s: %d '-- +goose Up' and %d '-- +goose Down' lines, want one of each", name, ups, downs)
		} else if upLine > downLine {
			t.Errorf("%s: the Down section comes before the Up section", name)
		}
	}
	if len(versions) == 0 {
		t.Fatalf("no migration files in %s", dir)
	}
	slices.Sort(versions)
	for i, v := range versions {
		if v != i+1 {
			t.Fatalf("versions %v: want 1..%d without gaps or duplicates", versions, len(versions))
		}
	}
}

// presence is one object a step of the round trip checks: query counts it.
type presence struct{ what, query string }

func table(name string) presence {
	return presence{"table " + name, `select count(*) from pg_class
		where relnamespace = 'public'::regnamespace and relkind = 'r' and relname = '` + name + `'`}
}

var (
	anyTable = presence{"any table", `select count(*) from pg_class where relnamespace = 'public'::regnamespace
		and relkind = 'r' and relname <> 'goose_db_version'`}
	seedTrigger  = presence{"trigger users_seed_default_categories", "select count(*) from pg_trigger where tgname = 'users_seed_default_categories'"}
	seedFunction = presence{"function app.seed_default_categories", `select count(*) from pg_proc
		where proname = 'seed_default_categories' and pronamespace = (select oid from pg_namespace where nspname = 'app')`}
	appSchema   = presence{"schema app", "select count(*) from pg_namespace where nspname = 'app'"}
	userIDFunc  = presence{"function app.current_user_id", "select count(*) from pg_proc where oid = to_regprocedure('app.current_user_id()')"}
	updatedFunc = presence{"function app.set_updated_at", "select count(*) from pg_proc where oid = to_regprocedure('app.set_updated_at()')"}
	// pg_roles is shared by the whole server, so this works from any database.
	appRoles = presence{"roles app_user, app_auth and app_login", `select (count(*) = 3)::int from pg_roles
		where rolname in ('app_user', 'app_auth', 'app_login')`}
)

// afterDown lists, per migration version, what must be gone and what must still be there once
// that migration is rolled back: each Down undoes exactly its own file. Rolling back 0001 is
// checked by checkNoLeftovers.
var afterDown = map[int64]struct{ gone, kept []presence }{
	4: {
		gone: []presence{table("transactions")},
		kept: []presence{table("categories"), table("users"), seedTrigger},
	},
	3: {
		gone: []presence{table("categories"), seedTrigger, seedFunction},
		kept: []presence{table("users"), table("profiles"), updatedFunc},
	},
	2: {
		gone: []presence{anyTable},
		kept: []presence{appSchema, userIDFunc, updatedFunc, appRoles},
	},
}

// Up, down one file at a time to nothing, and up again, in a throw-away database on the test
// server.
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

	checked := 0
	for range files {
		r, err := m.Down(ctx)
		if err != nil {
			t.Fatalf("down: %v", err)
		}
		step, ok := afterDown[r.Version]
		if ok {
			checked++
		}
		for _, p := range step.gone {
			if n := count(t, db, p.query); n != 0 {
				t.Errorf("after rolling back %s: %s is still there", r.Name, p.what)
			}
		}
		for _, p := range step.kept {
			if n := count(t, db, p.query); n == 0 {
				t.Errorf("after rolling back %s: %s is gone; an earlier file made it", r.Name, p.what)
			}
		}
	}
	if checked != len(afterDown) {
		t.Errorf("checked %d of the %d per-file down steps", checked, len(afterDown))
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
	if len(status) != len(files) {
		t.Errorf("status lists %d migrations, want %d", len(status), len(files))
	}
	for _, s := range status {
		if !s.Applied {
			t.Errorf("%s not applied after second up", s.Name)
		}
	}
}
