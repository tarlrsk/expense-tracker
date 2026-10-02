package schema

import (
	"testing"
	"time"
)

// Check constraints, not-null columns and locked columns (ADR-0039).
func TestRules(t *testing.T) {
	s := begin(t)
	a := s.newUser(t)
	food := s.category(t, a, "Food")
	tx := s.newTransaction(t, a, food)

	const insertTx = "insert into transactions (owner_id, amount, occurred_on, category_id, source) values ($1, $2, current_date, $3, $4)"

	t.Run("app_user", func(t *testing.T) {
		s.asUser(t, a)
		s.run(t, []attempt{
			{name: "amount zero", sql: insertTx, args: []any{a, "0", food, "manual"}, wantCode: checkViolation},
			{name: "amount negative", sql: insertTx, args: []any{a, "-5", food, "manual"}, wantCode: checkViolation},
			{name: "amount positive", sql: insertTx, args: []any{a, "0.01", food, "manual"}, wantRows: 1},
			{name: "update amount to zero", sql: "update transactions set amount = 0 where id = $1", args: []any{tx}, wantCode: checkViolation},
			{name: "unknown source", sql: insertTx, args: []any{a, "1", food, "email"}, wantCode: checkViolation},
			{name: "every known source", sql: "insert into transactions (owner_id, amount, occurred_on, category_id, source) select $1, 1, current_date, $2, s from unnest(array['manual', 'text', 'scan', 'csv']) s", args: []any{a, food}, wantRows: 4},
			{
				name: "no category", sql: "insert into transactions (owner_id, amount, occurred_on) values ($1, 1, current_date)",
				args: []any{a}, wantCode: notNullViolation,
			},
			{name: "unknown category", sql: insertTx, args: []any{a, "1", "00000000-0000-7000-8000-000000000000", "manual"}, wantCode: foreignKeyViolation},
			{name: "unknown kind", sql: "insert into categories (owner_id, name, kind) values ($1, 'Transfer', 'transfer')", args: []any{a}, wantCode: checkViolation},
			{name: "blank name", sql: "insert into categories (owner_id, name, kind) values ($1, '  ', 'expense')", args: []any{a}, wantCode: checkViolation},
			{name: "change kind", sql: "update categories set kind = 'income' where id = $1", args: []any{food}, wantCode: insufficientPrivilege},
			{name: "delete category", sql: "delete from categories where id = $1", args: []any{food}, wantCode: insufficientPrivilege},
		})
	})

	t.Run("category names", func(t *testing.T) {
		b := s.newUser(t)
		s.asUser(t, b)
		s.exec(t, "insert into categories (owner_id, name, kind) values ($1, 'Coffee', 'expense')", b)
		s.asUser(t, a)
		s.run(t, []attempt{
			{name: "same name as another user", sql: "insert into categories (owner_id, name, kind) values ($1, 'coffee', 'expense')", args: []any{a}, wantRows: 1},
			{name: "same name other case", sql: "insert into categories (owner_id, name, kind) values ($1, 'FOOD', 'income')", args: []any{a}, wantCode: uniqueViolation},
			{name: "rename onto existing", sql: "update categories set name = 'food' where owner_id = $1 and name = 'Health'", args: []any{a}, wantCode: uniqueViolation},
			{name: "archive the old one", sql: "update categories set archived = true where id = $1", args: []any{food}, wantRows: 1},
			{name: "name free again", sql: "insert into categories (owner_id, name, kind) values ($1, 'FOOD', 'expense')", args: []any{a}, wantRows: 1},
			{name: "unarchive clashes", sql: "update categories set archived = false where id = $1", args: []any{food}, wantCode: uniqueViolation},
		})
	})

	t.Run("app_auth", func(t *testing.T) {
		s.asAuth(t)
		s.run(t, []attempt{
			{name: "unknown role", sql: "update profiles set role = 'admin' where id = $1", args: []any{a}, wantCode: checkViolation},
			{name: "operator role", sql: "update profiles set role = 'operator' where id = $1", args: []any{a}, wantRows: 1},
			{
				name: "unknown purpose",
				sql:  "insert into email_tokens (user_id, purpose, token_hash, expires_at) values ($1, 'reset', '\\x01', now())",
				args: []any{a}, wantCode: checkViolation,
			},
			{name: "email case", sql: "insert into users (email) values ('Same@Example.test'), ('same@example.TEST')", wantCode: uniqueViolation},
		})
	})

	t.Run("updated_at", func(t *testing.T) {
		// now() is fixed for the whole transaction, so start from an old value.
		old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		s.asUser(t, a)
		var catID, txID string
		s.scan(t, "insert into categories (owner_id, name, kind, updated_at) values ($1, 'Pets', 'expense', $2) returning id", []any{a, old}, &catID)
		s.scan(t, "insert into transactions (owner_id, amount, occurred_on, category_id, updated_at) values ($1, 1, current_date, $2, $3) returning id", []any{a, catID, old}, &txID)
		s.exec(t, "update categories set icon = 'paw' where id = $1", catID)
		s.exec(t, "update transactions set note = 'vet' where id = $1", txID)
		for _, row := range []struct{ table, id string }{{"categories", catID}, {"transactions", txID}} {
			var got time.Time
			s.scan(t, "select updated_at from "+row.table+" where id = $1", []any{row.id}, &got)
			if !got.After(old) {
				t.Errorf("%s: updated_at %v did not move", row.table, got)
			}
		}
	})
}
