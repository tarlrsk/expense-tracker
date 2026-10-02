package schema

import (
	"slices"
	"testing"
)

// The API's two roles cannot log in and have no special powers (ADR-0019, ADR-0034).
func TestRoleAttributes(t *testing.T) {
	s := begin(t)
	for _, role := range []string{"app_user", "app_auth"} {
		t.Run(role, func(t *testing.T) {
			var login, super, bypassRLS, createRole, createDB, inherit, replication bool
			s.scan(t, `select rolcanlogin, rolsuper, rolbypassrls, rolcreaterole, rolcreatedb, rolinherit, rolreplication
				from pg_roles where rolname = $1`, []any{role},
				&login, &super, &bypassRLS, &createRole, &createDB, &inherit, &replication)
			for name, got := range map[string]bool{
				"login": login, "superuser": super, "bypassrls": bypassRLS, "createrole": createRole,
				"createdb": createDB, "inherit": inherit, "replication": replication,
			} {
				if got {
					t.Errorf("%s has %s", role, name)
				}
			}
			if n := s.count(t, "select count(*) from pg_auth_members where member = $1::regrole", role); n != 0 {
				t.Errorf("%s is a member of %d roles, want none", role, n)
			}
		})
	}
}

// The connecting owner may SET ROLE to both roles but does not inherit their rights.
func TestOwnerMembership(t *testing.T) {
	s := begin(t)
	for _, role := range []string{"app_user", "app_auth"} {
		t.Run(role, func(t *testing.T) {
			var canSet, inherits bool
			s.scan(t, `select coalesce(bool_or(set_option), false), coalesce(bool_or(inherit_option), false)
				from pg_auth_members where roleid = $1::regrole and member = current_user::regrole`,
				[]any{role}, &canSet, &inherits)
			if !canSet {
				t.Errorf("owner cannot SET ROLE %s", role)
			}
			if inherits {
				t.Errorf("owner inherits the rights of %s", role)
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
