package schema

import (
	"strings"
	"testing"
)

// app_user cannot touch users, sessions, email_tokens or login_attempts at all (ADR-0034).
func TestAppUserLockedOutOfAccountTables(t *testing.T) {
	s := begin(t)
	a := s.newUser(t)

	statements := map[string][]string{
		"users": {
			"select * from users",
			"insert into users (email) values ('x@example.test')",
			"update users set disabled_at = now()",
			"delete from users",
		},
		"sessions": {
			"select * from sessions",
			"insert into sessions (user_id, token_hash, expires_at) values ($1, '\\x01', now())",
			"update sessions set last_used_at = now()",
			"delete from sessions",
		},
		"email_tokens": {
			"select * from email_tokens",
			"insert into email_tokens (user_id, purpose, token_hash, expires_at) values ($1, 'set_password', '\\x01', now())",
			"update email_tokens set used_at = now()",
			"delete from email_tokens",
		},
		"login_attempts": {
			"select * from login_attempts",
			"insert into login_attempts (email, ip) values ('x@example.test', '127.0.0.1')",
			"update login_attempts set attempted_at = now()",
			"delete from login_attempts",
		},
	}
	s.asUser(t, a)
	for table, qs := range statements {
		t.Run(table, func(t *testing.T) {
			for _, q := range qs {
				var args []any
				if strings.Contains(q, "$1") {
					args = []any{a}
				}
				s.fails(t, insufficientPrivilege, q, args...)
			}
		})
	}
}

// app_auth can read and write the five account tables.
func TestAppAuthAccountTables(t *testing.T) {
	s := begin(t)
	s.asAuth(t)
	var user string
	s.scan(t, "insert into users (email) values (gen_random_uuid()::text || '@example.test') returning id", nil, &user)

	s.run(t, []attempt{
		{name: "select users", sql: "select * from users where id = $1", args: []any{user}, wantRows: 1},
		{name: "update users", sql: "update users set password_hash = 'h', disabled_at = now() where id = $1", args: []any{user}, wantRows: 1},

		{name: "insert profiles", sql: "insert into profiles (id) values ($1)", args: []any{user}, wantRows: 1},
		{name: "select profiles", sql: "select * from profiles where id = $1", args: []any{user}, wantRows: 1},
		{name: "update profiles", sql: "update profiles set display_name = 'A', role = 'operator' where id = $1", args: []any{user}, wantRows: 1},

		{
			name: "insert sessions", sql: "insert into sessions (user_id, token_hash, expires_at) values ($1, gen_random_uuid()::text::bytea, now() + interval '30 days')",
			args: []any{user}, wantRows: 1,
		},
		{name: "select sessions", sql: "select * from sessions where user_id = $1", args: []any{user}, wantRows: 1},
		{name: "update sessions", sql: "update sessions set last_used_at = now() where user_id = $1", args: []any{user}, wantRows: 1},

		{
			name: "insert email_tokens",
			sql:  "insert into email_tokens (user_id, purpose, token_hash, expires_at) values ($1, 'set_password', gen_random_uuid()::text::bytea, now() + interval '7 days')",
			args: []any{user}, wantRows: 1,
		},
		{name: "select email_tokens", sql: "select * from email_tokens where user_id = $1", args: []any{user}, wantRows: 1},
		{name: "update email_tokens", sql: "update email_tokens set used_at = now() where user_id = $1", args: []any{user}, wantRows: 1},

		{name: "insert login_attempts", sql: "insert into login_attempts (email, ip) values ('nobody@example.test', '192.0.2.1')", wantRows: 1},
		{name: "select login_attempts", sql: "select * from login_attempts where ip = '192.0.2.1'", wantRows: 1},
		{name: "update login_attempts", sql: "update login_attempts set attempted_at = now() where ip = '192.0.2.1'", wantRows: 1},

		{name: "delete login_attempts", sql: "delete from login_attempts where ip = '192.0.2.1'", wantRows: 1},
		{name: "delete email_tokens", sql: "delete from email_tokens where user_id = $1", args: []any{user}, wantRows: 1},
		{name: "delete sessions", sql: "delete from sessions where user_id = $1", args: []any{user}, wantRows: 1},
		{name: "delete profiles", sql: "delete from profiles where id = $1", args: []any{user}, wantRows: 1},
		{name: "delete users", sql: "delete from users where id = $1", args: []any{user}, wantRows: 1},
	})
}

// app_auth has no access at all to the financial tables (ADR-0034).
func TestAppAuthDeniedFinancialTables(t *testing.T) {
	s := begin(t)
	a := s.newUser(t)
	food := s.category(t, a, "Food")
	tx := s.newTransaction(t, a, food)

	s.asAuth(t)
	s.run(t, []attempt{
		{name: "select categories", sql: "select * from categories", wantCode: insufficientPrivilege},
		{
			name: "insert categories", sql: "insert into categories (owner_id, name, kind) values ($1, 'x', 'expense')",
			args: []any{a}, wantCode: insufficientPrivilege,
		},
		{name: "update categories", sql: "update categories set name = 'x' where id = $1", args: []any{food}, wantCode: insufficientPrivilege},
		{name: "delete categories", sql: "delete from categories where id = $1", args: []any{food}, wantCode: insufficientPrivilege},
		{name: "select transactions", sql: "select * from transactions", wantCode: insufficientPrivilege},
		{
			name: "insert transactions",
			sql:  "insert into transactions (owner_id, amount, occurred_on, category_id) values ($1, 1, current_date, $2)",
			args: []any{a, food}, wantCode: insufficientPrivilege,
		},
		{name: "update transactions", sql: "update transactions set note = 'x' where id = $1", args: []any{tx}, wantCode: insufficientPrivilege},
		{name: "delete transactions", sql: "delete from transactions where id = $1", args: []any{tx}, wantCode: insufficientPrivilege},
	})
}
