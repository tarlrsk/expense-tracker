package migrate_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

// The migration as Neon runs it: the database owner is not a superuser, has LOGIN, CREATEROLE and
// BYPASSRLS, and creates the app roles itself. The roles are renamed to throw-away names, because
// app_user and app_auth already exist on the test server (made by the superuser) and are in use.
// This also covers the branch the round trip never reaches: down drops the roles.
func TestNonSuperuserOwner(t *testing.T) {
	shared := dbtest.DB(t)
	cfg := dbtest.Config(t)
	suffix := randomHex(t, 6)
	owner, userRole, authRole := "t"+suffix+"_owner", "t"+suffix+"_user", "t"+suffix+"_auth"
	password := randomHex(t, 16)

	throwAwayRoles(t, shared, userRole, authRole, owner) // runs after the database is dropped
	if _, err := shared.ExecContext(t.Context(),
		"create role "+ident(owner)+" login nosuperuser createrole bypassrls password '"+password+"'"); err != nil {
		t.Fatalf("create owner role: %v", err)
	}
	dbName := throwAwayDB(t, shared, owner)

	ownerCfg := cfg.Copy()
	ownerCfg.User, ownerCfg.Password = owner, password
	db := connect(t, ownerCfg, dbName)
	var isSuper bool
	if err := db.QueryRowContext(t.Context(), "select rolsuper from pg_roles where rolname = current_user").Scan(&isSuper); err != nil || isSuper {
		t.Fatalf("not running as a non-superuser (super %v, err %v)", isSuper, err)
	}

	m, err := migrate.New(db, renamedMigrations(t, userRole, authRole))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Up(t.Context()); err != nil {
		t.Fatalf("up as non-superuser owner: %v", err)
	}

	t.Run("owner membership", func(t *testing.T) {
		for _, role := range []string{userRole, authRole} {
			var canSet, inherits bool
			if err := shared.QueryRowContext(t.Context(), `
				select coalesce(bool_or(set_option), false), coalesce(bool_or(inherit_option), false)
				from pg_auth_members where roleid = $1::regrole and member = $2::regrole`, role, owner).
				Scan(&canSet, &inherits); err != nil {
				t.Fatal(err)
			}
			if !canSet || inherits {
				t.Errorf("owner on %s: set %v, inherit %v; want set true, inherit false", role, canSet, inherits)
			}
		}
	})

	t.Run("roles, RLS and cascade", func(t *testing.T) {
		tx, err := db.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback() }()
		exec := func(q string, args ...any) {
			t.Helper()
			if _, err := tx.ExecContext(t.Context(), q, args...); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
		newUser := func() string {
			t.Helper()
			var id string
			if err := tx.QueryRowContext(t.Context(),
				"insert into users (email) values (gen_random_uuid()::text || '@example.test') returning id").Scan(&id); err != nil {
				t.Fatal(err)
			}
			exec("insert into profiles (id) values ($1)", id)
			return id
		}

		exec("set local role " + ident(authRole))
		a, b := newUser(), newUser()

		exec("set local role " + ident(userRole))
		exec("select set_config('app.user_id', $1, true)", a)
		if n := count(t, tx, "select count(*) from categories"); n != 13 {
			t.Errorf("A sees %d categories, want their 13", n)
		}
		if n := count(t, tx, "select count(*) from categories where owner_id = $1", b); n != 0 {
			t.Errorf("A sees %d of B's categories", n)
		}

		exec("set local role " + ident(authRole))
		exec("delete from users where id = $1", a)
		exec("set local role none")
		if n := count(t, tx, "select count(*) from categories where owner_id = $1", a); n != 0 {
			t.Errorf("%d categories of the removed user left", n)
		}
		if n := count(t, tx, "select count(*) from categories where owner_id = $1", b); n != 13 {
			t.Errorf("B has %d categories after A was removed, want 13", n)
		}
	})

	if _, err := m.Down(t.Context()); err != nil {
		t.Fatalf("down: %v", err)
	}
	checkNoLeftovers(t, db, "after down")
	if n := roleCount(t, shared, userRole, authRole); n != 0 {
		t.Errorf("after down: %d of the 2 throw-away roles left; down should drop roles no one uses", n)
	}

	if _, err := m.Up(t.Context()); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if n := roleCount(t, shared, userRole, authRole); n != 2 {
		t.Errorf("after second up: %d of the 2 roles exist", n)
	}
}

// A role that already exists with powers it must not have stops the migration, and the failed
// migration leaves nothing behind.
func TestPreexistingBadRole(t *testing.T) {
	shared := dbtest.DB(t)
	cfg := dbtest.Config(t)

	for _, tt := range []struct {
		name, attribute string
	}{
		{"bypassrls", "BYPASSRLS"},
		{"login", "LOGIN"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			suffix := randomHex(t, 6)
			userRole, authRole := "t"+suffix+"_user", "t"+suffix+"_auth"
			throwAwayRoles(t, shared, userRole, authRole)
			if _, err := shared.ExecContext(t.Context(),
				"create role "+ident(userRole)+" noinherit "+strings.ToLower(tt.attribute)); err != nil {
				t.Fatalf("create bad role: %v", err)
			}

			db := connect(t, cfg, throwAwayDB(t, shared, ""))
			m, err := migrate.New(db, renamedMigrations(t, userRole, authRole))
			if err != nil {
				t.Fatal(err)
			}
			applied, err := m.Up(t.Context())
			if err == nil {
				t.Fatal("up accepted a role with " + tt.attribute)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" ||
				!strings.Contains(pgErr.Message, userRole) || !strings.Contains(pgErr.Message, tt.attribute) {
				t.Errorf("up failed with %v; want the role check naming %s and %s", err, userRole, tt.attribute)
			}
			if len(applied) != 0 {
				t.Errorf("up reports %d applied migrations", len(applied))
			}

			checkNoLeftovers(t, db, "after the failed up")
			if n := roleCount(t, shared, authRole); n != 0 {
				t.Error("the failed up left the auth role behind")
			}
			status, err := m.Status(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range status {
				if s.Applied {
					t.Errorf("%s is marked applied", s.Name)
				}
			}
		})
	}
}
