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
			name: "insert as A", sql: "insert into categories (owner_id, name, kind) values ($1, 'Sneaky', 'expense')",
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

// Without app.user_id, app_user sees nothing and gets no error.
func TestNoUserSeesNothing(t *testing.T) {
	s := begin(t)
	a := s.newUser(t)
	// Inserted as the owner role, so this transaction never sets app.user_id before "never set".
	s.exec(t, `insert into transactions (owner_id, amount, occurred_on, category_id)
		values ($1, 1, current_date, $2)`, a, s.category(t, a, "Food"))

	checks := []attempt{
		{name: "select categories", sql: "select * from categories", wantRows: 0},
		{name: "select transactions", sql: "select * from transactions", wantRows: 0},
		{name: "select profiles", sql: "select * from profiles", wantRows: 0},
		{name: "update categories", sql: "update categories set icon = 'x'", wantRows: 0},
		{name: "delete transactions", sql: "delete from transactions", wantRows: 0},
		{
			name: "insert category", sql: "insert into categories (owner_id, name, kind) values ($1, 'x', 'expense')",
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
	txB := s.newTransaction(t, b, s.category(t, b, "Food"))

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
	})
}
