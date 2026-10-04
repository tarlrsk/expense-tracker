package schema

import "testing"

// User B can neither see nor change user A's categories (ADR-0014, ADR-0019).
func TestCategoriesCrossUser(t *testing.T) {
	s := begin(t)
	a, b := s.newUser(t), s.newUser(t)
	foodA := s.category(t, a, "Food")
	foodB := s.category(t, b, "Food")

	s.asUser(t, b)
	s.run(t, []attempt{
		{name: "select A's row", sql: "select * from categories where id = $1", args: []any{foodA}, wantRows: 0},
		{name: "select all sees only own", sql: "select * from categories", wantRows: 13},
		{name: "update A's row", sql: "update categories set name = 'Hacked' where id = $1", args: []any{foodA}, wantRows: 0},
		{name: "archive A's rows", sql: "update categories set archived = true where owner_id = $1", args: []any{a}, wantRows: 0},
		{name: "delete A's row", sql: "delete from categories where id = $1", args: []any{foodA}, wantCode: insufficientPrivilege},
		{
			name: "insert as A", sql: "insert into categories (owner_id, name, kind, sort_order) values ($1, 'Sneaky', 'expense', 14)",
			args: []any{a}, wantCode: insufficientPrivilege,
		},
		{name: "move own row to A", sql: "update categories set owner_id = $1 where id = $2", args: []any{a, foodB}, wantCode: insufficientPrivilege},
	})

	s.asOwner(t)
	var name string
	var archived bool
	s.scan(t, "select name, archived from categories where id = $1", []any{foodA}, &name, &archived)
	if name != "Food" || archived {
		t.Errorf("A's category changed: name %q, archived %v", name, archived)
	}
	if n := s.count(t, "select count(*) from categories where owner_id = $1", a); n != 13 {
		t.Errorf("A has %d categories, want 13", n)
	}
	if n := s.count(t, "select count(*) from categories where owner_id = $1 and id = $2", b, foodB); n != 1 {
		t.Errorf("B's category moved away")
	}
}

// User B can neither see nor change user A's transactions.
func TestTransactionsCrossUser(t *testing.T) {
	s := begin(t)
	a, b := s.newUser(t), s.newUser(t)
	foodA := s.category(t, a, "Food")
	txA := s.newTransaction(t, a, foodA)
	txB := s.newTransaction(t, b, s.category(t, b, "Food"))

	s.asUser(t, b)
	s.run(t, []attempt{
		{name: "select A's row", sql: "select * from transactions where id = $1", args: []any{txA}, wantRows: 0},
		{name: "select all sees only own", sql: "select * from transactions", wantRows: 1},
		{name: "update A's row", sql: "update transactions set note = 'hacked' where id = $1", args: []any{txA}, wantRows: 0},
		{name: "update all touches only own", sql: "update transactions set merchant = 'x'", wantRows: 1},
		{name: "delete A's row", sql: "delete from transactions where id = $1", args: []any{txA}, wantRows: 0},
		{
			name: "insert as A",
			sql:  "insert into transactions (owner_id, amount, occurred_on, category_id) values ($1, 1, current_date, $2)",
			args: []any{a, foodA}, wantCode: insufficientPrivilege,
		},
		{name: "move own row to A", sql: "update transactions set owner_id = $1 where id = $2", args: []any{a, txB}, wantCode: insufficientPrivilege},
	})

	s.asOwner(t)
	var note, merchant string
	s.scan(t, "select note, merchant from transactions where id = $1", []any{txA}, &note, &merchant)
	if note != "" || merchant != "" {
		t.Errorf("A's transaction changed: note %q, merchant %q", note, merchant)
	}
	if n := s.count(t, "select count(*) from transactions where owner_id = $1", a); n != 1 {
		t.Errorf("A has %d transactions, want 1", n)
	}
}

// User B can neither see nor change user A's merchant rules.
func TestMerchantRulesCrossUser(t *testing.T) {
	s := begin(t)
	a, b := s.newUser(t), s.newUser(t)
	foodA, foodB := s.category(t, a, "Food"), s.category(t, b, "Food")
	ruleA := s.newRule(t, a, foodA, "grab")
	ruleB := s.newRule(t, b, foodB, "grab")

	s.asUser(t, b)
	s.run(t, []attempt{
		{name: "select A's row", sql: "select * from merchant_rules where id = $1", args: []any{ruleA}, wantRows: 0},
		{name: "select all sees only own", sql: "select * from merchant_rules", wantRows: 1},
		{name: "update A's row", sql: "update merchant_rules set merchant = 'Hacked' where id = $1", args: []any{ruleA}, wantRows: 0},
		{name: "update all touches only own", sql: "update merchant_rules set merchant = 'Grab'", wantRows: 1},
		{name: "delete A's row", sql: "delete from merchant_rules where id = $1", args: []any{ruleA}, wantRows: 0},
		{
			name: "insert as A",
			sql:  "insert into merchant_rules (owner_id, merchant_key, merchant, category_id) values ($1, 'kfc', 'KFC', $2)",
			args: []any{a, foodA}, wantCode: insufficientPrivilege,
		},
		{name: "move own row to A", sql: "update merchant_rules set owner_id = $1 where id = $2", args: []any{a, ruleB}, wantCode: insufficientPrivilege},
	})

	s.asOwner(t)
	var merchant string
	s.scan(t, "select merchant from merchant_rules where id = $1", []any{ruleA}, &merchant)
	if merchant != "grab" {
		t.Errorf("A's rule changed: merchant %q", merchant)
	}
	if n := s.count(t, "select count(*) from merchant_rules where owner_id = $1", a); n != 1 {
		t.Errorf("A has %d rules, want 1", n)
	}
}

// User B can neither see nor change user A's AI usage rows; nobody can delete them.
func TestAIUsageCrossUser(t *testing.T) {
	s := begin(t)
	a, b := s.newUser(t), s.newUser(t)
	s.newUsage(t, a, "2026-10-01")
	s.newUsage(t, b, "2026-10-01")

	s.asUser(t, b)
	s.run(t, []attempt{
		{name: "select A's row", sql: "select * from ai_usage where owner_id = $1", args: []any{a}, wantRows: 0},
		{name: "select all sees only own", sql: "select * from ai_usage", wantRows: 1},
		{name: "update A's row", sql: "update ai_usage set parse_count = 0 where owner_id = $1", args: []any{a}, wantRows: 0},
		{name: "update all touches only own", sql: "update ai_usage set parse_count = parse_count + 1", wantRows: 1},
		{name: "delete A's row", sql: "delete from ai_usage where owner_id = $1", args: []any{a}, wantCode: insufficientPrivilege},
		{
			name: "insert as A", sql: "insert into ai_usage (owner_id, day) values ($1, date '2026-10-02')",
			args: []any{a}, wantCode: insufficientPrivilege,
		},
		{
			name: "move own row to A", sql: "update ai_usage set owner_id = $1 where owner_id = $2",
			args: []any{a, b}, wantCode: insufficientPrivilege,
		},
	})

	s.asOwner(t)
	if n := s.count(t, "select parse_count from ai_usage where owner_id = $1 and day = date '2026-10-01'", a); n != 1 {
		t.Errorf("A's parse_count = %d, want 1", n)
	}
	if n := s.count(t, "select count(*) from ai_usage where owner_id = $1", b); n != 1 {
		t.Errorf("B has %d usage rows, want 1", n)
	}
}

// Without app.user_id, app_user sees nothing and gets no error.
func TestNoUserSeesNothing(t *testing.T) {
	s := begin(t)
	a := s.newUser(t)
	// Inserted as the owner role, so this transaction never sets app.user_id before "never set".
	s.exec(t, `insert into transactions (owner_id, amount, occurred_on, category_id)
		values ($1, 1, current_date, $2)`, a, s.category(t, a, "Food"))
	s.exec(t, `insert into merchant_rules (owner_id, merchant_key, merchant, category_id)
		values ($1, 'grab', 'Grab', $2)`, a, s.category(t, a, "Food"))
	s.exec(t, "insert into ai_usage (owner_id, day) values ($1, current_date)", a)

	checks := []attempt{
		{name: "select categories", sql: "select * from categories", wantRows: 0},
		{name: "select transactions", sql: "select * from transactions", wantRows: 0},
		{name: "select profiles", sql: "select * from profiles", wantRows: 0},
		{name: "select merchant_rules", sql: "select * from merchant_rules", wantRows: 0},
		{name: "select ai_usage", sql: "select * from ai_usage", wantRows: 0},
		{name: "update categories", sql: "update categories set icon = 'x'", wantRows: 0},
		{name: "delete transactions", sql: "delete from transactions", wantRows: 0},
		{
			name: "insert category", sql: "insert into categories (owner_id, name, kind, sort_order) values ($1, 'x', 'expense', 14)",
			args: []any{a}, wantCode: insufficientPrivilege,
		},
	}

	t.Run("never set", func(t *testing.T) {
		// This transaction has not set app.user_id (the pooled connection may have, in an earlier
		// transaction; then it reads back as '').
		s.exec(t, "set local role app_user")
		s.run(t, checks)
	})
	t.Run("empty", func(t *testing.T) {
		s.asUser(t, "")
		var isNull bool
		s.scan(t, "select app.current_user_id() is null", nil, &isNull)
		if !isNull {
			t.Error("app.current_user_id() is not null for ''")
		}
		s.run(t, checks)
	})
}

// Foreign-key checks ignore RLS, so the owner-scoped key is what stops B pointing at A's
// category (ADR-0039).
func TestOwnerScopedForeignKey(t *testing.T) {
	s := begin(t)
	a, b := s.newUser(t), s.newUser(t)
	foodA := s.category(t, a, "Food")
	foodB := s.category(t, b, "Food")
	txB := s.newTransaction(t, b, foodB)
	ruleB := s.newRule(t, b, foodB, "grab")

	s.asUser(t, b)
	s.run(t, []attempt{
		{
			name: "create with A's category",
			sql:  "insert into transactions (owner_id, amount, occurred_on, category_id) values ($1, 1, current_date, $2)",
			args: []any{b, foodA}, wantCode: foreignKeyViolation,
		},
		{
			name: "update to A's category", sql: "update transactions set category_id = $1 where id = $2",
			args: []any{foodA, txB}, wantCode: foreignKeyViolation,
		},
		{
			name: "rule with A's category",
			sql:  "insert into merchant_rules (owner_id, merchant_key, merchant, category_id) values ($1, 'kfc', 'KFC', $2)",
			args: []any{b, foodA}, wantCode: foreignKeyViolation,
		},
		{
			name: "rule moved to A's category", sql: "update merchant_rules set category_id = $1 where id = $2",
			args: []any{foodA, ruleB}, wantCode: foreignKeyViolation,
		},
	})
}
