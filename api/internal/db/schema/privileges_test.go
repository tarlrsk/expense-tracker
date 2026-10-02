package schema

import (
	"slices"
	"strings"
	"testing"
)

// grants is the complete, exact set of privileges that app_user, app_auth, app_login and PUBLIC
// hold on the tables, views and sequences of schema public (goose's version table included).
// app_login holds none: it reaches data only after switching to app_user or app_auth. Anything not
// written here must not exist: a later `grant truncate ...` or `grant all ...` fails the test.
// Keys are "table" for table-level privileges and "table.column" for column-level ones.
var grants = map[string]map[string][]string{
	"users":          {"app_auth": {"SELECT", "INSERT", "UPDATE", "DELETE"}},
	"sessions":       {"app_auth": {"SELECT", "INSERT", "UPDATE", "DELETE"}},
	"email_tokens":   {"app_auth": {"SELECT", "INSERT", "UPDATE", "DELETE"}},
	"login_attempts": {"app_auth": {"SELECT", "INSERT", "UPDATE", "DELETE"}},
	"profiles": {
		"app_auth": {"SELECT", "INSERT", "UPDATE", "DELETE"},
		"app_user": {"SELECT"},
	},
	"profiles.display_name": {"app_user": {"UPDATE"}},

	"categories":            {"app_user": {"SELECT", "INSERT"}},
	"categories.name":       {"app_user": {"UPDATE"}},
	"categories.icon":       {"app_user": {"UPDATE"}},
	"categories.archived":   {"app_user": {"UPDATE"}},
	"categories.sort_order": {"app_user": {"UPDATE"}},
	"categories.updated_at": {"app_user": {"UPDATE"}},

	"transactions":             {"app_user": {"SELECT", "INSERT", "DELETE"}},
	"transactions.amount":      {"app_user": {"UPDATE"}},
	"transactions.currency":    {"app_user": {"UPDATE"}},
	"transactions.occurred_on": {"app_user": {"UPDATE"}},
	"transactions.merchant":    {"app_user": {"UPDATE"}},
	"transactions.category_id": {"app_user": {"UPDATE"}},
	"transactions.note":        {"app_user": {"UPDATE"}},
	"transactions.updated_at":  {"app_user": {"UPDATE"}},
}

func TestPrivilegeMatrix(t *testing.T) {
	var want []string
	for object, byRole := range grants {
		for role, privs := range byRole {
			for _, p := range privs {
				want = append(want, object+" "+role+" "+p)
			}
		}
	}
	slices.Sort(want)

	s := begin(t)
	// Direct grants are all these roles have: app_user and app_auth are members of no role, and
	// app_login does not inherit from its two (TestMemberships), so their rights are their own
	// grants plus PUBLIC's.
	got := s.strings(t, `
		with grantee(oid, name) as (
			values (0::oid, 'public'), ('app_user'::regrole::oid, 'app_user'), ('app_auth'::regrole::oid, 'app_auth'),
				('app_login'::regrole::oid, 'app_login')
		), rel as (
			select oid, relname, relacl from pg_class
			where relnamespace = 'public'::regnamespace and relkind in ('r', 'p', 'v', 'm', 'S', 'f')
		)
		select rel.relname || ' ' || g.name || ' ' || a.privilege_type
		from rel cross join lateral aclexplode(rel.relacl) a join grantee g on g.oid = a.grantee
		union all
		select rel.relname || '.' || att.attname || ' ' || g.name || ' ' || a.privilege_type
		from rel join pg_attribute att on att.attrelid = rel.oid and att.attnum > 0 and not att.attisdropped
		cross join lateral aclexplode(att.attacl) a join grantee g on g.oid = a.grantee
		order by 1`)

	for _, g := range got {
		if !slices.Contains(want, g) {
			t.Errorf("unexpected privilege: %s", g)
		}
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("missing privilege: %s", w)
		}
	}
}

func TestFunctionGuards(t *testing.T) {
	s := begin(t)

	t.Run("security definer functions pin search_path", func(t *testing.T) {
		const userSchemas = `n.nspname not in ('pg_catalog', 'information_schema') and n.nspname not like 'pg\_%'`
		loose := s.strings(t, `select p.oid::regprocedure::text from pg_proc p join pg_namespace n on n.oid = p.pronamespace
			where `+userSchemas+` and p.prosecdef
			  and not exists (select from unnest(coalesce(p.proconfig, '{}'::text[])) c where c like 'search_path=%')`)
		if len(loose) > 0 {
			t.Errorf("SECURITY DEFINER functions without search_path: %v", loose)
		}
		if n := s.count(t, `select count(*) from pg_proc p join pg_namespace n on n.oid = p.pronamespace
			where `+userSchemas+` and p.prosecdef`); n == 0 {
			t.Error("no SECURITY DEFINER function found; the check above proves nothing")
		}
	})

	t.Run("app functions closed to public and app_login", func(t *testing.T) {
		for _, role := range []string{"public", "app_login"} {
			open := s.strings(t, `select oid::regprocedure::text from pg_proc
				where pronamespace = 'app'::regnamespace and has_function_privilege($1, oid, 'EXECUTE')`, role)
			if len(open) > 0 {
				t.Errorf("functions in app executable by %s: %v", role, open)
			}
		}
	})

	t.Run("new functions start closed", func(t *testing.T) {
		// Runs as the migrating role, inside the rolled-back transaction.
		s.exec(t, "create function app.probe_default_privileges() returns int language sql as 'select 1'")
		var open bool
		s.scan(t, "select has_function_privilege('public', 'app.probe_default_privileges()', 'EXECUTE')", nil, &open)
		if open {
			t.Error("a new function is executable by PUBLIC; the default privileges were not changed")
		}
	})

	t.Run("schemas", func(t *testing.T) {
		for _, c := range []struct {
			role, schema, priv string
			want               bool
		}{
			{"app_user", "public", "CREATE", false},
			{"app_user", "app", "CREATE", false},
			{"app_auth", "public", "CREATE", false},
			{"app_auth", "app", "CREATE", false},
			{"public", "public", "CREATE", false},
			{"public", "app", "CREATE", false},
			{"app_auth", "app", "USAGE", false},
			{"public", "app", "USAGE", false},
			{"app_login", "public", "CREATE", false},
			{"app_login", "app", "CREATE", false},
			{"app_login", "app", "USAGE", false},
			{"app_user", "app", "USAGE", true},
		} {
			var got bool
			s.scan(t, "select has_schema_privilege($1, $2, $3)", []any{c.role, c.schema, c.priv}, &got)
			if got != c.want {
				t.Errorf("%s %s on schema %s = %v, want %v", c.role, strings.ToLower(c.priv), c.schema, got, c.want)
			}
		}
	})
}
