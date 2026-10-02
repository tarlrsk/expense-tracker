package migrate_test

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

// The migration as Neon runs it: the database owner is not a superuser, has LOGIN, CREATEROLE and
// BYPASSRLS, and creates the app roles itself. The roles are renamed to throw-away names, because
// app_user, app_auth and app_login already exist on the test server (made by the superuser) and
// are in use. This also covers the branch the round trip never reaches: down drops the roles.
//
// The owner no longer switches roles. The RLS and cascade checks log in as the renamed login
// role, with a password the owner sets on it as `make db-login-password` does.
func TestNonSuperuserOwner(t *testing.T) {
	shared := dbtest.DB(t)
	cfg := dbtest.Config(t)
	roles := newTestRoles(t)
	owner := strings.TrimSuffix(roles.user, "_user") + "_owner"
	password := randomHex(t, 16)

	throwAwayRoles(t, shared, append(roles.all(), owner)...) // runs after the database is dropped
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

	m, err := migrate.New(db, renamedMigrations(t, roles))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Up(t.Context()); err != nil {
		t.Fatalf("up as non-superuser owner: %v", err)
	}

	t.Run("owner cannot switch", func(t *testing.T) {
		for _, role := range []string{roles.user, roles.auth} {
			if n := count(t, shared, `select count(*) from pg_auth_members
				where roleid = $1::regrole and member = $2::regrole and (set_option or inherit_option)`, role, owner); n != 0 {
				t.Errorf("owner has a SET or INHERIT membership in %s", role)
			}
		}
	})

	t.Run("login role memberships", func(t *testing.T) {
		got := memberships(t, shared, roles.login)
		want := []string{roles.auth + " inherit=false set=true admin=false", roles.user + " inherit=false set=true admin=false"}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("memberships = %v, want %v", got, want)
		}
	})

	// The owner sets the login role's password, as make db-login-password does on Neon.
	loginPassword := randomHex(t, 16)
	if _, err := db.ExecContext(t.Context(), "alter role "+ident(roles.login)+" password '"+loginPassword+"'"); err != nil {
		t.Fatalf("owner sets the login role's password: %v", err)
	}
	loginCfg := cfg.Copy()
	loginCfg.User, loginCfg.Password = roles.login, loginPassword
	login := connect(t, loginCfg, dbName)

	t.Run("login role sees nothing by itself", func(t *testing.T) {
		for _, table := range []string{"users", "categories", "transactions"} {
			var n int
			err := login.QueryRowContext(t.Context(), "select count(*) from "+table).Scan(&n) //nolint:gosec // fixed table names
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
				t.Errorf("%s as the login role: %v, want permission denied", table, err)
			}
		}
	})

	t.Run("roles, RLS and cascade", func(t *testing.T) {
		rlsAndCascade(t, login, roles)
	})

	// The roles are dropped by the last down (0001); close the login role's connections first.
	if err := login.Close(); err != nil {
		t.Fatal(err)
	}
	downAll(t, m)
	checkNoLeftovers(t, db, "after down")
	if n := roleCount(t, shared, roles.all()...); n != 0 {
		t.Errorf("after down: %d of the 3 throw-away roles left; down should drop roles no one uses", n)
	}

	if _, err := m.Up(t.Context()); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if n := roleCount(t, shared, roles.all()...); n != 3 {
		t.Errorf("after second up: %d of the 3 roles exist", n)
	}
}

// rlsAndCascade works as the login role, switching roles exactly as WithUserTx and WithAuthTx do.
func rlsAndCascade(t *testing.T, login *sql.DB, roles testRoles) {
	t.Helper()
	tx, err := login.BeginTx(t.Context(), nil)
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
	asUser := func(id string) {
		t.Helper()
		exec("set local role " + ident(roles.user))
		exec("select set_config('app.user_id', $1, true)", id)
	}

	exec("set local role " + ident(roles.auth))
	a, b := newUser(), newUser()

	asUser(a)
	if n := count(t, tx, "select count(*) from categories"); n != 13 {
		t.Errorf("A sees %d categories, want their 13", n)
	}
	if n := count(t, tx, "select count(*) from categories where owner_id = $1", b); n != 0 {
		t.Errorf("A sees %d of B's categories", n)
	}

	exec("set local role " + ident(roles.auth))
	exec("delete from users where id = $1", a)
	// Seen through RLS: the removed user's categories are gone, B's are untouched.
	asUser(a)
	if n := count(t, tx, "select count(*) from categories"); n != 0 {
		t.Errorf("%d categories of the removed user left", n)
	}
	asUser(b)
	if n := count(t, tx, "select count(*) from categories"); n != 13 {
		t.Errorf("B has %d categories after A was removed, want 13", n)
	}
}

// memberships lists the roles member belongs to, with the membership's options, sorted.
func memberships(t *testing.T, shared *sql.DB, member string) []string {
	t.Helper()
	rows, err := shared.QueryContext(t.Context(), `select m.roleid::regrole::text || ' inherit=' || m.inherit_option
			|| ' set=' || m.set_option || ' admin=' || m.admin_option
		from pg_auth_members m where m.member = $1::regrole order by 1`, member)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// A role that already exists with powers it must not have stops the migration in its first file,
// and the failed run leaves nothing behind: no object, no role, no goose version row.
func TestPreexistingBadRole(t *testing.T) {
	shared := dbtest.DB(t)
	cfg := dbtest.Config(t)

	for _, tt := range []struct {
		name      string
		pick      func(testRoles) string // which role exists beforehand
		create    string                 // its attributes
		attribute string                 // what the error must name
	}{
		{"app_user with bypassrls", func(r testRoles) string { return r.user }, "noinherit bypassrls", "BYPASSRLS"},
		{"app_user with login", func(r testRoles) string { return r.user }, "noinherit login", "LOGIN"},
		{"app_login with bypassrls", func(r testRoles) string { return r.login }, "noinherit login bypassrls", "BYPASSRLS"},
		{"app_login without login", func(r testRoles) string { return r.login }, "noinherit nologin", "NOLOGIN"},
		{"app_login inheriting", func(r testRoles) string { return r.login }, "inherit login", "INHERIT"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			roles := newTestRoles(t)
			bad := tt.pick(roles)
			throwAwayRoles(t, shared, roles.all()...)
			if _, err := shared.ExecContext(t.Context(), "create role "+ident(bad)+" "+tt.create); err != nil {
				t.Fatalf("create bad role: %v", err)
			}

			db := connect(t, cfg, throwAwayDB(t, shared, ""))
			m, err := migrate.New(db, renamedMigrations(t, roles))
			if err != nil {
				t.Fatal(err)
			}
			applied, err := m.Up(t.Context())
			if err == nil {
				t.Fatal("up accepted a role with " + tt.attribute)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" ||
				!strings.Contains(pgErr.Message, bad) || !strings.Contains(pgErr.Message, tt.attribute) {
				t.Errorf("up failed with %v; want the role check naming %s and %s", err, bad, tt.attribute)
			}
			if len(applied) != 0 {
				t.Errorf("up reports %d applied migrations", len(applied))
			}
			// The role check is in the first file: nothing after it may have run.
			var partial *goose.PartialError
			if !errors.As(err, &partial) || partial.Failed == nil || partial.Failed.Source.Version != 1 {
				t.Errorf("up failed with %v; want it to fail in migration 1", err)
			}
			// goose's own baseline row (version 0) is all its table may hold.
			if n := count(t, db, "select count(*) from goose_db_version where version_id <> 0"); n != 0 {
				t.Errorf("the failed up left %d goose version rows beyond the baseline", n)
			}

			checkNoLeftovers(t, db, "after the failed up")
			if n := roleCount(t, shared, roles.all()...); n != 1 {
				t.Errorf("the failed up left %d roles; want only the one that existed before", n)
			}
			if bad == roles.login {
				if got := memberships(t, shared, roles.login); len(got) != 0 {
					t.Errorf("the failed up left memberships of the login role: %v", got)
				}
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
