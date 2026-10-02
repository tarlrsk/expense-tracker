package schema

import "testing"

// Removing a user as app_auth deletes all their data through `on delete cascade`, including
// categories and transactions on which app_auth has no rights (ADR-0034; PLAN-0002 open question).
// Another user's rows stay.
func TestRemoveUserCascades(t *testing.T) {
	s := begin(t)
	a, b := s.newUser(t), s.newUser(t)
	for _, u := range []string{a, b} {
		s.asAuth(t)
		s.exec(t, `insert into sessions (user_id, token_hash, expires_at)
			values ($1, gen_random_uuid()::text::bytea, now() + interval '30 days')`, u)
		s.exec(t, `insert into email_tokens (user_id, purpose, token_hash, expires_at)
			values ($1, 'set_password', gen_random_uuid()::text::bytea, now() + interval '7 days')`, u)
		s.newTransaction(t, u, s.category(t, u, "Food"))
	}

	s.asAuth(t)
	if n := s.exec(t, "delete from users where id = $1", a); n != 1 {
		t.Fatalf("deleted %d users, want 1", n)
	}

	s.asOwner(t)
	counts := []struct {
		table, query string
		wantB        int // rows of B; A must have none
	}{
		{"users", "select count(*) from users where id = $1", 1},
		{"profiles", "select count(*) from profiles where id = $1", 1},
		{"sessions", "select count(*) from sessions where user_id = $1", 1},
		{"email_tokens", "select count(*) from email_tokens where user_id = $1", 1},
		{"categories", "select count(*) from categories where owner_id = $1", 13},
		{"transactions", "select count(*) from transactions where owner_id = $1", 1},
	}
	for _, c := range counts {
		if n := s.count(t, c.query, a); n != 0 {
			t.Errorf("%s: %d rows of the removed user left", c.table, n)
		}
		if n := s.count(t, c.query, b); n != c.wantB {
			t.Errorf("%s: other user has %d rows, want %d", c.table, n, c.wantB)
		}
	}
}
