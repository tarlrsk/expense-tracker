package schema

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
)

// SQLSTATE codes the tests expect.
const (
	insufficientPrivilege = "42501" // permission denied, including a row-level security violation
	foreignKeyViolation   = "23503"
	uniqueViolation       = "23505"
	checkViolation        = "23514"
	notNullViolation      = "23502"
)

// session is one transaction on the test database that is rolled back when the test ends.
type session struct {
	tx *sql.Tx
}

func begin(t *testing.T) *session {
	t.Helper()
	return &session{tx: dbtest.Tx(t, dbtest.DB(t))}
}

// asOwner goes back to the connecting role (the migration owner; it bypasses RLS).
func (s *session) asOwner(t *testing.T) { t.Helper(); s.exec(t, "set local role none") }

// asAuth switches to app_auth, as WithAuthTx will.
func (s *session) asAuth(t *testing.T) { t.Helper(); s.exec(t, "set local role app_auth") }

// asUser switches to app_user acting for userID, as WithUserTx will.
func (s *session) asUser(t *testing.T, userID string) {
	t.Helper()
	s.exec(t, "set local role app_user")
	s.exec(t, "select set_config('app.user_id', $1, true)", userID)
}

// exec runs q, fails the test on error and returns the number of rows affected.
func (s *session) exec(t *testing.T, q string, args ...any) int64 {
	t.Helper()
	res, err := s.tx.ExecContext(t.Context(), q, args...)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("%s: rows affected: %v", q, err)
	}
	return n
}

// scan runs a query that returns one row and scans it into dest.
func (s *session) scan(t *testing.T, q string, args []any, dest ...any) {
	t.Helper()
	if err := s.tx.QueryRowContext(t.Context(), q, args...).Scan(dest...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

func (s *session) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	s.scan(t, q, args, &n)
	return n
}

func (s *session) strings(t *testing.T, q string, args ...any) []string {
	t.Helper()
	rows, err := s.tx.QueryContext(t.Context(), q, args...)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return out
}

// try runs q inside a savepoint, so a failure does not end the transaction, and returns the
// SQLSTATE of the error ("" on success) and the rows affected.
func (s *session) try(t *testing.T, q string, args ...any) (code string, rows int64) {
	t.Helper()
	s.exec(t, "savepoint try")
	res, err := s.tx.ExecContext(t.Context(), q, args...)
	if err == nil {
		rows, err = res.RowsAffected()
	}
	if err != nil {
		s.exec(t, "rollback to savepoint try")
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Fatalf("%s: not a Postgres error: %v", q, err)
		}
		return pgErr.Code, 0
	}
	s.exec(t, "release savepoint try")
	return "", rows
}

// fails checks that q fails with SQLSTATE code; the change, if any, is undone.
func (s *session) fails(t *testing.T, code, q string, args ...any) {
	t.Helper()
	got, _ := s.try(t, q, args...)
	if got != code {
		t.Errorf("%s: SQLSTATE %q, want %q", q, got, code)
	}
}

// newUser creates an account as app_auth (the profile included, as Go will) and returns its id.
// The default categories come from the trigger. The role is the owner again afterwards.
func (s *session) newUser(t *testing.T) string {
	t.Helper()
	s.asAuth(t)
	var id string
	s.scan(t, "insert into users (email) values (gen_random_uuid()::text || '@example.test') returning id", nil, &id)
	s.exec(t, "insert into profiles (id) values ($1)", id)
	s.asOwner(t)
	return id
}

// category returns the id of the owner's category with that name (read as the owner role).
func (s *session) category(t *testing.T, owner, name string) string {
	t.Helper()
	s.asOwner(t)
	var id string
	s.scan(t, "select id from categories where owner_id = $1 and name = $2 and not archived", []any{owner, name}, &id)
	return id
}

// newTransaction inserts a transaction as app_user for owner and returns its id.
// The role is the owner again afterwards.
func (s *session) newTransaction(t *testing.T, owner, categoryID string) string {
	t.Helper()
	s.asUser(t, owner)
	var id string
	s.scan(t, `insert into transactions (owner_id, amount, occurred_on, category_id)
		values ($1, 120.50, date '2026-10-01', $2) returning id`, []any{owner, categoryID}, &id)
	s.asOwner(t)
	return id
}

// attempt is one statement and its expected outcome: an SQLSTATE, or success with a row count.
type attempt struct {
	name     string
	sql      string
	args     []any
	wantCode string // "" means it must succeed
	wantRows int64  // checked only on success
}

// run executes every attempt in the current role and checks its outcome.
func (s *session) run(t *testing.T, attempts []attempt) {
	t.Helper()
	for _, a := range attempts {
		code, rows := s.try(t, a.sql, a.args...)
		switch {
		case code != a.wantCode:
			t.Errorf("%s: SQLSTATE %q, want %q", a.name, code, a.wantCode)
		case code == "" && rows != a.wantRows:
			t.Errorf("%s: %d rows, want %d", a.name, rows, a.wantRows)
		}
	}
}
