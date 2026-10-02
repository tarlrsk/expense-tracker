package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotMigrated: the database has no tables yet.
var ErrNotMigrated = errors.New("the database is not migrated (table public.users or public.transactions is missing); run make migrate")

// ErrRoleTooPowerful: the role the API logged in as could read data without WithUserTx or
// WithAuthTx, or cannot switch to app_user and app_auth.
var ErrRoleTooPowerful = errors.New("the database role in DATABASE_URL is not usable by the API; " +
	"it must log in as app_login (make db-login-password prints the line for .env)")

// checkSQL looks at the login role (session_user) and the role in effect (current_user; the
// same unless the connection string sets a role). For each it reports whether it is, or belongs
// to, a superuser or BYPASSRLS role; whether it can read users or transactions as it is; whether
// it owns, or belongs to the owner of, either table; and whether it can SET ROLE to app_user
// and app_auth.
const checkSQL = `
with checked as (
  select r.oid, r.rolname from pg_catalog.pg_roles r
  where r.rolname in (session_user, current_user)
), tables(oid) as (
  values ('public.users'::regclass), ('public.transactions'::regclass)
)
select c.rolname,
  exists (select from pg_catalog.pg_roles p
          where (p.rolsuper or p.rolbypassrls) and pg_catalog.pg_has_role(c.oid, p.oid, 'MEMBER')),
  exists (select from tables t where pg_catalog.has_table_privilege(c.oid, t.oid, 'SELECT')),
  exists (select from tables t join pg_catalog.pg_class k on k.oid = t.oid
          where pg_catalog.pg_has_role(c.oid, k.relowner, 'MEMBER')),
  pg_catalog.pg_has_role(c.oid, 'app_user', 'SET') and pg_catalog.pg_has_role(c.oid, 'app_auth', 'SET')
from checked c
order by c.rolname`

// Check refuses a database the API must not use: one that is not migrated, or a login role
// that could read data outside WithUserTx / WithAuthTx or cannot switch roles. Open runs it.
// Its errors name roles, never the connection string.
func (d *DB) Check(ctx context.Context) error {
	var migrated bool
	if err := d.sqlDB.QueryRowContext(ctx, `select
		to_regclass('public.users') is not null and to_regclass('public.transactions') is not null
		and to_regrole('app_user') is not null and to_regrole('app_auth') is not null`).Scan(&migrated); err != nil {
		return fmt.Errorf("check the database: %w", err)
	}
	if !migrated {
		return ErrNotMigrated
	}

	rows, err := d.sqlDB.QueryContext(ctx, checkSQL)
	if err != nil {
		return fmt.Errorf("check the database role: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var problems []string
	for rows.Next() {
		var role string
		var powerful, canRead, owns, canSwitch bool
		if err := rows.Scan(&role, &powerful, &canRead, &owns, &canSwitch); err != nil {
			return fmt.Errorf("check the database role: %w", err)
		}
		if powerful {
			problems = append(problems, fmt.Sprintf("role %s is, or belongs to, a superuser or BYPASSRLS role", role))
		}
		if canRead {
			problems = append(problems, fmt.Sprintf("role %s can read public.users or public.transactions without switching roles", role))
		}
		if owns {
			problems = append(problems, fmt.Sprintf("role %s owns, or belongs to the owner of, the tables", role))
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
