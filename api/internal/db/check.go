package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotMigrated: the database lacks a table or role of the migrations.
var ErrNotMigrated = errors.New("the database is not migrated; run make migrate")

// ErrRoleTooPowerful: the role the API logged in as could read data without WithUserTx or
// WithAuthTx, has powers the API must not have, or cannot switch to app_user and app_auth.
var ErrRoleTooPowerful = errors.New("the database role in DATABASE_URL is not usable by the API; " +
	"it must log in as app_login (make db-login-password prints the line for .env)")

// migratedTables are every table of the migrations; migratedRoles the roles the API switches to.
var (
	migratedTables = []string{
		"public.users", "public.profiles", "public.sessions", "public.email_tokens", "public.login_attempts",
		"public.categories", "public.transactions",
	}
	migratedRoles = []string{"app_user", "app_auth"}
)

// missingSQL lists the tables ($1) and roles ($2) that do not exist.
const missingSQL = `
select coalesce(string_agg(m, ', '), '') from (
  select t as m from unnest($1::text[]) t where pg_catalog.to_regclass(t) is null
  union all
  select 'role ' || r from unnest($2::text[]) r where pg_catalog.to_regrole(r) is null
) missing`

// checkSQL looks at the login role (session_user) and the role in effect (current_user; the
// same unless the connection string sets a role), with the same rule migration 0001 applies to
// app_login. For each it reports:
//   - whether it is, or belongs to, a superuser or BYPASSRLS role;
//   - which of the attributes REPLICATION, CREATEROLE and CREATEDB it has;
//   - its memberships other than exactly SET (without INHERIT and ADMIN) on app_user and
//     app_auth;
//   - which tables ($1) it can read as it is, and which it owns or belongs to the owner of;
//   - whether it can SET ROLE to app_user and app_auth.
const checkSQL = `
with checked as (
  select r.oid, r.rolname, r.rolreplication, r.rolcreaterole, r.rolcreatedb from pg_catalog.pg_roles r
  where r.rolname in (session_user, current_user)
), tables as (
  select t as name, pg_catalog.to_regclass(t) as oid from unnest($1::text[]) t
)
select c.rolname,
  exists (select from pg_catalog.pg_roles p
          where (p.rolsuper or p.rolbypassrls) and pg_catalog.pg_has_role(c.oid, p.oid, 'MEMBER')),
  pg_catalog.concat_ws(', ',
    case when c.rolreplication then 'REPLICATION' end,
    case when c.rolcreaterole  then 'CREATEROLE' end,
    case when c.rolcreatedb    then 'CREATEDB' end),
  coalesce((select string_agg(distinct pg_catalog.quote_ident(g.rolname), ', ')
            from pg_catalog.pg_auth_members m join pg_catalog.pg_roles g on g.oid = m.roleid
            where m.member = c.oid
              and (g.rolname not in ('app_user', 'app_auth')
                   or m.inherit_option or not m.set_option or m.admin_option)), ''),
  coalesce((select string_agg(t.name, ', ' order by t.name) from tables t
            where pg_catalog.has_table_privilege(c.oid, t.oid, 'SELECT')), ''),
  coalesce((select string_agg(t.name, ', ' order by t.name) from tables t
            join pg_catalog.pg_class k on k.oid = t.oid
            where pg_catalog.pg_has_role(c.oid, k.relowner, 'MEMBER')), ''),
  pg_catalog.pg_has_role(c.oid, 'app_user', 'SET') and pg_catalog.pg_has_role(c.oid, 'app_auth', 'SET')
from checked c
order by c.rolname`

// Check refuses a database the API must not use: one that is not migrated, or a login role
// that could read data outside WithUserTx / WithAuthTx, has powers beyond switching to app_user
// and app_auth, or cannot switch. Open runs it. Its errors name roles and tables, never the
// connection string.
func (d *DB) Check(ctx context.Context) error {
	var missing string
	if err := d.sqlDB.QueryRowContext(ctx, missingSQL, migratedTables, migratedRoles).Scan(&missing); err != nil {
		return fmt.Errorf("check the database: %w", err)
	}
	if missing != "" {
		return fmt.Errorf("%w (missing: %s)", ErrNotMigrated, missing)
	}

	rows, err := d.sqlDB.QueryContext(ctx, checkSQL, migratedTables)
	if err != nil {
		return fmt.Errorf("check the database role: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var problems []string
	for rows.Next() {
		var role, attributes, memberships, readable, owned string
		var powerful, canSwitch bool
		if err := rows.Scan(&role, &powerful, &attributes, &memberships, &readable, &owned, &canSwitch); err != nil {
			return fmt.Errorf("check the database role: %w", err)
		}
		if powerful {
			problems = append(problems, fmt.Sprintf("role %s is, or belongs to, a superuser or BYPASSRLS role", role))
		}
		if attributes != "" {
			problems = append(problems, fmt.Sprintf("role %s has %s", role, attributes))
		}
		if memberships != "" {
			problems = append(problems, fmt.Sprintf("role %s has memberships other than SET without INHERIT or ADMIN "+
				"on app_user and app_auth: %s", role, memberships))
		}
		if readable != "" {
			problems = append(problems, fmt.Sprintf("role %s can read %s without switching roles", role, readable))
		}
		if owned != "" {
			problems = append(problems, fmt.Sprintf("role %s owns, or belongs to the owner of, %s", role, owned))
		}
		if !canSwitch {
			problems = append(problems, fmt.Sprintf("role %s cannot SET ROLE to app_user and app_auth", role))
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("check the database role: %w", err)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrRoleTooPowerful, strings.Join(problems, "; "))
	}
	return nil
}
