package schema

import "testing"

// app_user reads and renames only their own profile and can never change role (ADR-0019, ADR-0035).
func TestProfiles(t *testing.T) {
	s := begin(t)
	a, b := s.newUser(t), s.newUser(t)

	s.asUser(t, a)
	s.run(t, []attempt{
		{name: "select sees only own", sql: "select * from profiles", wantRows: 1},
		{name: "select own", sql: "select * from profiles where id = $1", args: []any{a}, wantRows: 1},
		{name: "select other", sql: "select * from profiles where id = $1", args: []any{b}, wantRows: 0},
		{name: "rename own", sql: "update profiles set display_name = 'Alice' where id = $1", args: []any{a}, wantRows: 1},
		{name: "rename other", sql: "update profiles set display_name = 'Mallory' where id = $1", args: []any{b}, wantRows: 0},
		{name: "change own role", sql: "update profiles set role = 'operator' where id = $1", args: []any{a}, wantCode: insufficientPrivilege},
		{name: "change other's role", sql: "update profiles set role = 'operator' where id = $1", args: []any{b}, wantCode: insufficientPrivilege},
		{name: "change own id", sql: "update profiles set id = $1 where id = $2", args: []any{b, a}, wantCode: insufficientPrivilege},
		{name: "insert", sql: "insert into profiles (id) values ($1)", args: []any{a}, wantCode: insufficientPrivilege},
		{name: "delete own", sql: "delete from profiles where id = $1", args: []any{a}, wantCode: insufficientPrivilege},
	})

	s.asOwner(t)
	for _, tt := range []struct{ id, wantName string }{{a, "Alice"}, {b, ""}} {
		var name, role string
		s.scan(t, "select display_name, role from profiles where id = $1", []any{tt.id}, &name, &role)
		if name != tt.wantName || role != "user" {
			t.Errorf("profile %s: display_name %q, role %q; want %q, user", tt.id, name, role, tt.wantName)
		}
	}
}
