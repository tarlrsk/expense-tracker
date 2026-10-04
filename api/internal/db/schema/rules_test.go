package schema

import (
	"strings"
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
	const insertTxCurrency = "insert into transactions (owner_id, amount, occurred_on, category_id, currency) values ($1, 1, current_date, $2, $3)"
	const insertCategory = "insert into categories (owner_id, name, kind, sort_order) values ($1, $2, 'expense', $3)"

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
			{name: "unknown kind", sql: "insert into categories (owner_id, name, kind, sort_order) values ($1, 'Transfer', 'transfer', 14)", args: []any{a}, wantCode: checkViolation},
			{name: "blank name", sql: insertCategory, args: []any{a, "  ", 14}, wantCode: checkViolation},
			{name: "empty name", sql: insertCategory, args: []any{a, "", 14}, wantCode: checkViolation},
			{name: "leading space", sql: insertCategory, args: []any{a, " Tea", 14}, wantCode: checkViolation},
			{name: "trailing space", sql: insertCategory, args: []any{a, "Tea ", 14}, wantCode: checkViolation},
			{name: "rename with space", sql: "update categories set name = 'Food ' where id = $1", args: []any{food}, wantCode: checkViolation},
			{name: "negative sort_order", sql: insertCategory, args: []any{a, "Pets", -1}, wantCode: checkViolation},
			{name: "update sort_order negative", sql: "update categories set sort_order = -1 where id = $1", args: []any{food}, wantCode: checkViolation},
			{name: "no sort_order", sql: "insert into categories (owner_id, name, kind) values ($1, 'Pets', 'expense')", args: []any{a}, wantCode: notNullViolation},
			{name: "currency THB", sql: insertTxCurrency, args: []any{a, food, "THB"}, wantRows: 1},
			{name: "currency USD", sql: insertTxCurrency, args: []any{a, food, "USD"}, wantCode: checkViolation},
			{name: "currency us", sql: insertTxCurrency, args: []any{a, food, "us"}, wantCode: checkViolation},
			{name: "currency thb", sql: insertTxCurrency, args: []any{a, food, "thb"}, wantCode: checkViolation},
			{name: "update currency to USD", sql: "update transactions set currency = 'USD' where id = $1", args: []any{tx}, wantCode: checkViolation},
			{name: "change kind", sql: "update categories set kind = 'income' where id = $1", args: []any{food}, wantCode: insufficientPrivilege},
			{name: "delete category", sql: "delete from categories where id = $1", args: []any{food}, wantCode: insufficientPrivilege},
		})
	})

	t.Run("category names", func(t *testing.T) {
		b := s.newUser(t)
		s.asUser(t, b)
		s.exec(t, "insert into categories (owner_id, name, kind, sort_order) values ($1, 'Coffee', 'expense', 14)", b)
		s.asUser(t, a)
		s.run(t, []attempt{
			{name: "same name as another user", sql: "insert into categories (owner_id, name, kind, sort_order) values ($1, 'coffee', 'expense', 14)", args: []any{a}, wantRows: 1},
			{name: "same name other case", sql: "insert into categories (owner_id, name, kind, sort_order) values ($1, 'FOOD', 'income', 14)", args: []any{a}, wantCode: uniqueViolation},
			{name: "rename onto existing", sql: "update categories set name = 'food' where owner_id = $1 and name = 'Health'", args: []any{a}, wantCode: uniqueViolation},
			{name: "archive the old one", sql: "update categories set archived = true where id = $1", args: []any{food}, wantRows: 1},
			{name: "name free again", sql: "insert into categories (owner_id, name, kind, sort_order) values ($1, 'FOOD', 'expense', 14)", args: []any{a}, wantRows: 1},
			{name: "unarchive clashes", sql: "update categories set archived = false where id = $1", args: []any{food}, wantCode: uniqueViolation},
		})
	})

	t.Run("sort order", func(t *testing.T) {
		b := s.newUser(t)
		foodB := s.category(t, b, "Food")
		s.asUser(t, a)
		s.run(t, []attempt{
			{name: "own category", sql: "update categories set sort_order = 20 where id = $1", args: []any{food}, wantRows: 1},
			{name: "ties allowed", sql: "update categories set sort_order = 20 where owner_id = $1 and name = 'Health'", args: []any{a}, wantRows: 1},
			{name: "other user's category", sql: "update categories set sort_order = 0 where id = $1", args: []any{foodB}, wantRows: 0},
		})
		s.asOwner(t)
		if n := s.count(t, "select sort_order from categories where id = $1", foodB); n != 1 {
			t.Errorf("B's Food sort_order = %d, want 1", n)
		}
	})

	t.Run("merchant rules", func(t *testing.T) {
		b := s.newUser(t)
		s.newRule(t, b, s.category(t, b, "Food"), "kfc")
		rule := s.newRule(t, a, food, "grab")
		health := s.category(t, a, "Health")
		long := strings.Repeat("ก", 100) // 100 characters, 300 bytes: the limit counts characters
		const insertRule = "insert into merchant_rules (owner_id, merchant_key, merchant, category_id) values ($1, $2, $3, $4)"
		s.asUser(t, a)
		s.run(t, []attempt{
			{name: "empty key", sql: insertRule, args: []any{a, "", "X", food}, wantCode: checkViolation},
			{name: "blank key", sql: insertRule, args: []any{a, "  ", "X", food}, wantCode: checkViolation},
			{name: "key leading space", sql: insertRule, args: []any{a, " x", "X", food}, wantCode: checkViolation},
			{name: "key trailing space", sql: insertRule, args: []any{a, "x ", "X", food}, wantCode: checkViolation},
			{name: "key too long", sql: insertRule, args: []any{a, long + "x", "X", food}, wantCode: checkViolation},
			{name: "empty merchant", sql: insertRule, args: []any{a, "x", "", food}, wantCode: checkViolation},
			{name: "blank merchant", sql: insertRule, args: []any{a, "x", "  ", food}, wantCode: checkViolation},
			{name: "merchant leading space", sql: insertRule, args: []any{a, "x", " X", food}, wantCode: checkViolation},
			{name: "merchant trailing space", sql: insertRule, args: []any{a, "x", "X ", food}, wantCode: checkViolation},
			{name: "merchant too long", sql: insertRule, args: []any{a, "x", long + "x", food}, wantCode: checkViolation},
			{name: "100 characters", sql: insertRule, args: []any{a, long, long, food}, wantRows: 1},
			{
				name: "no category", sql: "insert into merchant_rules (owner_id, merchant_key, merchant) values ($1, 'x', 'X')",
				args: []any{a}, wantCode: notNullViolation,
			},
			{name: "unknown category", sql: insertRule, args: []any{a, "x", "X", "00000000-0000-7000-8000-000000000000"}, wantCode: foreignKeyViolation},
			{name: "duplicate key", sql: insertRule, args: []any{a, "grab", "Grab", health}, wantCode: uniqueViolation},
			{name: "key of another user", sql: insertRule, args: []any{a, "kfc", "KFC", food}, wantRows: 1},
			{name: "update merchant", sql: "update merchant_rules set merchant = 'Grab Food' where id = $1", args: []any{rule}, wantRows: 1},
			{name: "update merchant with space", sql: "update merchant_rules set merchant = 'Grab ' where id = $1", args: []any{rule}, wantCode: checkViolation},
			{name: "update category", sql: "update merchant_rules set category_id = $2 where id = $1", args: []any{rule, health}, wantRows: 1},
			{name: "update key", sql: "update merchant_rules set merchant_key = 'grabfood' where id = $1", args: []any{rule}, wantCode: insufficientPrivilege},
			{name: "update id", sql: "update merchant_rules set id = uuidv7() where id = $1", args: []any{rule}, wantCode: insufficientPrivilege},
			{name: "update created_at", sql: "update merchant_rules set created_at = now() where id = $1", args: []any{rule}, wantCode: insufficientPrivilege},
			{name: "delete rule", sql: "delete from merchant_rules where id = $1", args: []any{rule}, wantRows: 1},
		})
	})

	t.Run("ai usage", func(t *testing.T) {
		b := s.newUser(t)
		s.newUsage(t, b, "2026-10-01")
		s.newUsage(t, a, "2026-10-01")
		const insertUsage = "insert into ai_usage (owner_id, day, parse_count) values ($1, $2, $3)"
		s.asUser(t, a)
		s.run(t, []attempt{
			{name: "negative count", sql: insertUsage, args: []any{a, "2026-10-02", -1}, wantCode: checkViolation},
			{name: "no day", sql: "insert into ai_usage (owner_id) values ($1)", args: []any{a}, wantCode: notNullViolation},
			{name: "duplicate day", sql: insertUsage, args: []any{a, "2026-10-01", 0}, wantCode: uniqueViolation},
			{name: "next day", sql: insertUsage, args: []any{a, "2026-10-02", 0}, wantRows: 1},
			{name: "update count", sql: "update ai_usage set parse_count = parse_count + 1 where owner_id = $1", args: []any{a}, wantRows: 2},
			{name: "update count negative", sql: "update ai_usage set parse_count = -1 where owner_id = $1", args: []any{a}, wantCode: checkViolation},
			{
				// How the API is expected to count a call: one statement, whether or not today's row exists.
				name: "count by upsert",
				sql: `insert into ai_usage (owner_id, day, parse_count) values ($1, date '2026-10-01', 1)
					on conflict (owner_id, day) do update set parse_count = ai_usage.parse_count + 1`,
				args: []any{a}, wantRows: 1,
			},
			{name: "update day", sql: "update ai_usage set day = date '2026-09-30' where owner_id = $1", args: []any{a}, wantCode: insufficientPrivilege},
			{name: "update owner", sql: "update ai_usage set owner_id = $1 where owner_id = $1", args: []any{a}, wantCode: insufficientPrivilege},
			{name: "delete usage", sql: "delete from ai_usage where owner_id = $1", args: []any{a}, wantCode: insufficientPrivilege},
		})
		s.asOwner(t)
		if n := s.count(t, "select parse_count from ai_usage where owner_id = $1 and day = date '2026-10-01'", a); n != 3 {
			t.Errorf("A's parse_count = %d, want 3 (1, +1, +1 by upsert)", n)
		}
		if n := s.count(t, "select parse_count from ai_usage where owner_id = $1", b); n != 1 {
			t.Errorf("B's parse_count = %d, want 1", n)
		}
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
		var catID, txID, ruleID string
		s.scan(t, "insert into categories (owner_id, name, kind, sort_order, updated_at) values ($1, 'Pets', 'expense', 14, $2) returning id", []any{a, old}, &catID)
		s.scan(t, "insert into transactions (owner_id, amount, occurred_on, category_id, updated_at) values ($1, 1, current_date, $2, $3) returning id", []any{a, catID, old}, &txID)
		s.scan(t, "insert into merchant_rules (owner_id, merchant_key, merchant, category_id, updated_at) values ($1, 'vet', 'Vet', $2, $3) returning id", []any{a, catID, old}, &ruleID)
		s.exec(t, "update categories set icon = 'paw' where id = $1", catID)
		s.exec(t, "update transactions set note = 'vet' where id = $1", txID)
		s.exec(t, "update merchant_rules set merchant = 'The Vet' where id = $1", ruleID)
		rows := []struct{ table, id string }{{"categories", catID}, {"transactions", txID}, {"merchant_rules", ruleID}}
		for _, row := range rows {
			var got time.Time
			s.scan(t, "select updated_at from "+row.table+" where id = $1", []any{row.id}, &got)
			if !got.After(old) {
				t.Errorf("%s: updated_at %v did not move", row.table, got)
			}
		}

		// app_user may send updated_at (GORM does), but the trigger's now() wins.
		s.exec(t, "update categories set updated_at = $2, icon = 'cat' where id = $1", catID, old)
		s.exec(t, "update transactions set updated_at = $2, note = 'cat' where id = $1", txID, old)
		s.exec(t, "update merchant_rules set updated_at = $2, merchant = 'Cat Vet' where id = $1", ruleID, old)
		for _, row := range rows {
			var isNow bool
			s.scan(t, "select updated_at = now() from "+row.table+" where id = $1", []any{row.id}, &isNow)
			if !isNow {
				t.Errorf("%s: updated_at sent by app_user was kept; want now()", row.table)
			}
		}
	})
}
