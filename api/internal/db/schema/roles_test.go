package schema

import (
	"slices"
	"testing"
)

// The API's two working roles cannot log in and have no special powers (ADR-0019, ADR-0034).
func TestRoleAttributes(t *testing.T) {
	s := begin(t)
	for _, role := range []string{"app_user", "app_auth", "app_login"} {
		t.Run(role, func(t *testing.T) {
			var login, super, bypassRLS, createRole, createDB, inherit, replication bool
			s.scan(t, `select rolcanlogin, rolsuper, rolbypassrls, rolcreaterole, rolcreatedb, rolinherit, rolreplication
				from pg_roles where rolname = $1`, []any{role},
				&login, &super, &bypassRLS, &createRole, &createDB, &inherit, &replication)
			// Only app_login, the role the API logs in as, may log in.
			if want := role == "app_login"; login != want {
				t.Errorf("%s login = %v, want %v", role, login, want)
			}
			for name, got := range map[string]bool{
				"superuser": super, "bypassrls": bypassRLS, "createrole": createRole,
				"createdb": createDB, "inherit": inherit, "replication": replication,
			} {
				if got {
					t.Errorf("%s has %s", role, name)
				}
			}
		})
	}
}

// Memberships: app_login may SET ROLE to app_user and app_auth and nothing else, without
// inheriting their rights or administering them; app_user and app_auth belong to nobody.
func TestMemberships(t *testing.T) {
	s := begin(t)
	got := s.strings(t, `select m.roleid::regrole::text || ' inherit=' || m.inherit_option || ' set=' || m.set_option
			|| ' admin=' || m.admin_option
		from pg_auth_members m where m.member = 'app_login'::regrole order by 1`)
	want := []string{"app_auth inherit=false set=true admin=false", "app_user inherit=false set=true admin=false"}
	if !slices.Equal(got, want) {
		t.Errorf("app_login memberships = %v, want %v", got, want)
	}
	for _, role := range []string{"app_user", "app_auth"} {
		if n := s.count(t, "select count(*) from pg_auth_members where member = $1::regrole", role); n != 0 {
			t.Errorf("%s is a member of %d roles, want none", role, n)
		}
	}
}

// The connecting owner (the migration role) cannot SET ROLE to app_user or app_auth through a
// membership: only migrations use it, and they never switch (the API logs in as app_login).
func TestOwnerMembership(t *testing.T) {
	s := begin(t)
	for _, role := range []string{"app_user", "app_auth"} {
		t.Run(role, func(t *testing.T) {
			if n := s.count(t, `select count(*) from pg_auth_members
				where roleid = $1::regrole and member = current_user::regrole and (set_option or inherit_option)`, role); n != 0 {
				t.Errorf("owner has a SET or INHERIT membership in %s", role)
			}
		})
	}
}

// accountTables are the only tables app_auth may touch (ADR-0034).
var accountTables = []string{"users", "profiles", "sessions", "email_tokens", "login_attempts"}

// gooseTable is goose's version table; it is not part of the app schema.
const gooseTable = "goose_db_version"

// Guards for every future migration: each table has RLS, app_auth reaches only account tables,
// app_user reaches no account table except profiles.
func TestSchemaGuards(t *testing.T) {
	s := begin(t)
	tables := s.strings(t, `select relname from pg_class
		where relnamespace = 'public'::regnamespace and relkind in ('r', 'p') order by relname`)
	for _, want := range []string{"users", "profiles", "sessions", "email_tokens", "login_attempts", "categories", "transactions"} {
		if !slices.Contains(tables, want) {
			t.Fatalf("table %s missing; tables: %v", want, tables)
		}
	}

	t.Run("rls enabled everywhere", func(t *testing.T) {
		without := s.strings(t, `select relname from pg_class
			where relnamespace = 'public'::regnamespace and relkind in ('r', 'p')
			  and relname <> $1 and not relrowsecurity order by relname`, gooseTable)
		if len(without) > 0 {
			t.Errorf("tables without row-level security: %v", without)
		}
	})

	t.Run("app_auth only on account tables", func(t *testing.T) {
		for _, table := range tables {
			if slices.Contains(accountTables, table) {
				continue
			}
			if privs := privileges(t, s, "app_auth", table); len(privs) > 0 {
				t.Errorf("app_auth has %v on %s", privs, table)
			}
		}
	})

	// RLS would also stop some of these updates; the column grants must stop them on their own.
	t.Run("locked columns", func(t *testing.T) {
		for _, c := range []struct{ table, column string }{
			{"profiles", "id"}, {"profiles", "role"}, {"profiles", "created_at"},
			{"categories", "id"}, {"categories", "owner_id"}, {"categories", "kind"}, {"categories", "created_at"},
			{"transactions", "id"}, {"transactions", "owner_id"}, {"transactions", "source"},
			{"transactions", "raw_input"}, {"transactions", "created_at"},
		} {
			var canUpdate bool
			s.scan(t, "select has_column_privilege('app_user', ('public.' || quote_ident($1))::regclass, $2, 'UPDATE')",
				[]any{c.table, c.column}, &canUpdate)
			if canUpdate {
				t.Errorf("app_user can update %s.%s", c.table, c.column)
			}
		}
	})

	t.Run("app_login on no table", func(t *testing.T) {
		for _, table := range tables {
			if privs := privileges(t, s, "app_login", table); len(privs) > 0 {
				t.Errorf("app_login has %v on %s", privs, table)
			}
		}
	})

	t.Run("app_user not on account tables", func(t *testing.T) {
		for _, table := range []string{"users", "sessions", "email_tokens", "login_attempts", gooseTable} {
			if privs := privileges(t, s, "app_user", table); len(privs) > 0 {
				t.Errorf("app_user has %v on %s", privs, table)
			}
		}
	})
}

// privileges lists every table-level and column-level privilege role has on public.table.
func privileges(t *testing.T, s *session, role, table string) []string {
	t.Helper()
	return s.strings(t, `
		select 'table ' || p from unnest(array['SELECT', 'INSERT', 'UPDATE', 'DELETE', 'TRUNCATE',
			'REFERENCES', 'TRIGGER', 'MAINTAIN']) p
		where has_table_privilege($1, ('public.' || quote_ident($2))::regclass, p)
		union all
		select 'column ' || p from unnest(array['SELECT', 'INSERT', 'UPDATE', 'REFERENCES']) p
		where has_any_column_privilege($1, ('public.' || quote_ident($2))::regclass, p)`, role, table)
}
