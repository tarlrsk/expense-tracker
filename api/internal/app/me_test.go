package app

import (
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// API-level tests of GET, PATCH and DELETE /api/me (PLAN-0002 T6; ADR-0038, ADR-0068).

func (e *apiEnv) displayName(a account) string {
	e.t.Helper()
	var name string
	if err := e.super.QueryRowContext(e.t.Context(), "select display_name from profiles where id = $1", a.id).Scan(&name); err != nil {
		e.t.Fatal(err)
	}
	return name
}

func TestGetMe(t *testing.T) {
	e := newAPIEnv(t)
	for _, opts := range []accountOpts{{}, {operator: true}} {
		a := e.newAccount(opts)
		e.exec("update profiles set display_name = 'Ann' where id = $1", a.id)
		rec := e.do(http.MethodGet, "/api/me", e.freshSession(a.id), "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/me: %d %s", rec.Code, rec.Body.String())
		}
		m := decodeObject(t, rec)
		assertKeys(t, "profile", m, "id", "email", "display_name", "role", "created_at")
		wantRole := "user"
		if opts.operator {
			wantRole = "operator"
		}
		if m["id"] != a.id.String() || m["email"] != a.email || m["display_name"] != "Ann" || m["role"] != wantRole {
			t.Errorf("profile = %v", m)
		}
		if d := time.Since(parseTime(t, m["created_at"])); d < -time.Minute || d > time.Minute {
			t.Errorf("created_at %v is not about now", m["created_at"])
		}
	}
}

func TestUpdateMe(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	patch := func(body string) (int, map[string]any) {
		rec := e.do(http.MethodPatch, "/api/me", tok, body)
		if rec.Code != http.StatusOK {
			return rec.Code, map[string]any{"error": errorCode(t, rec)}
		}
		return rec.Code, decodeObject(t, rec)
	}

	t.Run("trims, allows 0 to 50 characters, and returns the profile", func(t *testing.T) {
		for _, c := range []struct{ in, want string }{
			{"  Ann Lee \t", "Ann Lee"},
			{strings.Repeat("ก", 50), strings.Repeat("ก", 50)},
			{"   ", ""},
			{"Bea", "Bea"},
		} {
			code, m := patch(jsonBody(t, map[string]string{"display_name": c.in}))
			if code != http.StatusOK {
				t.Fatalf("PATCH %q: %d %v", c.in, code, m)
			}
			assertKeys(t, "profile", m, "id", "email", "display_name", "role", "created_at")
			if m["display_name"] != c.want || m["id"] != a.id.String() || m["role"] != "user" {
				t.Errorf("PATCH %q: %v, want display_name %q", c.in, m, c.want)
			}
			if got := e.displayName(a); got != c.want {
				t.Errorf("stored %q, want %q", got, c.want)
			}
		}
	})

	t.Run("longer than 50 characters is invalid_input and changes nothing", func(t *testing.T) {
		code, m := patch(jsonBody(t, map[string]string{"display_name": strings.Repeat("ก", 51)}))
		if code != http.StatusBadRequest || m["error"] != "invalid_input" {
			t.Errorf("51 characters: %d %v", code, m)
		}
		if got := e.displayName(a); got != "Bea" {
			t.Errorf("stored %q, want Bea", got)
		}
	})

	t.Run("a missing or null display_name leaves it", func(t *testing.T) {
		for _, body := range []string{`{}`, `{"display_name":null}`} {
			code, m := patch(body)
			if code != http.StatusOK || m["display_name"] != "Bea" {
				t.Errorf("PATCH %s: %d %v", body, code, m)
			}
		}
	})

	t.Run("role, email and other fields are refused", func(t *testing.T) {
		for _, body := range []string{
			`{"role":"operator"}`,
			`{"email":"other@example.test"}`,
			`{"display_name":"Mallory","role":"operator"}`,
			`{"display_name":"Mallory","email":"other@example.test"}`,
			`{"id":"` + a.id.String() + `"}`,
			`{"display_name":5}`,
			``,
		} {
			code, m := patch(body)
			if code != http.StatusBadRequest || m["error"] != "invalid_input" {
				t.Errorf("PATCH %s: %d %v, want 400 invalid_input", body, code, m)
			}
		}
		if n := e.count("select count(*) from users u join profiles p on p.id = u.id "+
			"where u.id = $1 and u.email = $2 and p.role = 'user' and p.display_name = 'Bea'", a.id, a.email); n != 1 {
			t.Error("a refused PATCH changed the account")
		}
	})
}

func TestDeleteMe(t *testing.T) {
	e := newAPIEnv(t)

	t.Run("a wrong password is 400, counted and limited", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		for i := range domain.MaxFailuresPerEmail {
			rec := e.deleteMe(tok, "not the password")
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" ||
				!strings.Contains(rec.Body.String(), domain.WrongPasswordMessage) {
				t.Fatalf("wrong password %d: %d %s", i+1, rec.Code, rec.Body.String())
			}
		}
		if n := e.attemptsForEmail(a.email); n != domain.MaxFailuresPerEmail {
			t.Errorf("attempts = %d, want %d", n, domain.MaxFailuresPerEmail)
		}
		if rec := e.deleteMe(tok, a.password); rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "rate_limited" {
			t.Errorf("right password at the limit: %d %s, want 429", rec.Code, rec.Body.String())
		}
		if rec := e.login(a.email, a.password); rec.Code != http.StatusTooManyRequests {
			t.Errorf("login at the limit: %d, want 429", rec.Code)
		}
		if n := e.count("select count(*) from users where id = $1", a.id); n != 1 {
			t.Error("the account was deleted")
		}
		if code, _ := e.whoami(tok); code != http.StatusOK {
			t.Errorf("session after failures: %d, want 200", code)
		}
	})

	t.Run("the right password deletes the account and all its rows", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok, other := e.freshSession(a.id), e.freshSession(a.id)
		e.newLink(a.id, time.Now().Add(time.Hour), false)
		e.login(a.email, "one earlier failure")
		if rec := e.deleteMe(tok, a.password); rec.Code != http.StatusNoContent {
			t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
		}
		e.assertGone(a.id)
		for _, s := range []string{tok, other} {
			if code, _ := e.whoami(s); code != http.StatusUnauthorized {
				t.Errorf("session after delete: %d, want 401", code)
			}
		}
		if n := e.attemptsForEmail(a.email); n != 1 {
			t.Errorf("attempts = %d, want 1 (the success is not counted)", n)
		}
		if rec := e.login(a.email, a.password); rec.Code != http.StatusUnauthorized {
			t.Errorf("login after delete: %d, want 401", rec.Code)
		}
	})

	t.Run("the last operator cannot delete their account; one of two can", func(t *testing.T) {
		e := e.with(t)
		op := e.newAccount(accountOpts{operator: true})
		e.onlyOperators(op.id)
		tok := e.freshSession(op.id)
		rec := e.deleteMe(tok, op.password)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), domain.LastOperatorMessage) {
			t.Errorf("last operator: %d %s, want 409", rec.Code, rec.Body.String())
		}
		if n := e.count("select count(*) from users where id = $1", op.id); n != 1 {
			t.Fatal("the last operator was deleted")
		}
		e.newAccount(accountOpts{operator: true})
		if rec := e.deleteMe(tok, op.password); rec.Code != http.StatusNoContent {
			t.Errorf("one of two operators: %d %s, want 204", rec.Code, rec.Body.String())
		}
		e.assertGone(op.id)
	})

	t.Run("bad input", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		for name, body := range map[string]string{
			"no body":       ``,
			"unknown field": `{"password":"` + a.password + `","user_id":"` + a.id.String() + `"}`,
			"wrong type":    `{"password":5}`,
		} {
			rec := e.do(http.MethodDelete, "/api/me", tok, body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("%s: %d %s, want 400", name, rec.Code, rec.Body.String())
			}
		}
		if n := e.count("select count(*) from users where id = $1", a.id); n != 1 {
			t.Error("the account was deleted")
		}
	})
}

// /api/me without a usable token is 401 on every method.
func TestMeNeedsSession(t *testing.T) {
	e := newAPIEnv(t)
	for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
		for _, tok := range []string{"", domain.NewToken().Plain} {
			rec := e.do(method, "/api/me", tok, `{"display_name":"x","password":"y"}`)
			if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "unauthenticated" {
				t.Errorf("%s /api/me with token %q: %d %s, want 401", method, tok, rec.Code, rec.Body.String())
			}
		}
	}
}

// A's GET, PATCH and DELETE /api/me never read or change B (ADR-0014).
func TestMeCrossUser(t *testing.T) {
	e := newAPIEnv(t)
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)
	e.exec("update profiles set display_name = 'Bea' where id = $1", b.id)
	bRows := e.rowsOf(b.id)

	rec := e.do(http.MethodGet, "/api/me", aTok, "")
	if m := decodeObject(t, rec); m["id"] != a.id.String() || m["email"] != a.email || strings.Contains(rec.Body.String(), b.email) {
		t.Errorf("A's GET: %s", rec.Body.String())
	}

	rec = e.do(http.MethodPatch, "/api/me", aTok, `{"display_name":"Changed"}`)
	if m := decodeObject(t, rec); rec.Code != http.StatusOK || m["id"] != a.id.String() || m["display_name"] != "Changed" {
		t.Errorf("A's PATCH: %d %s", rec.Code, rec.Body.String())
	}
	if got := e.displayName(b); got != "Bea" {
		t.Errorf("B's display name = %q after A's PATCH", got)
	}

	if rec := e.deleteMe(aTok, b.password); rec.Code != http.StatusBadRequest {
		t.Errorf("A's DELETE with B's password: %d, want 400", rec.Code)
	}
	if rec := e.deleteMe(aTok, a.password); rec.Code != http.StatusNoContent {
		t.Fatalf("A's DELETE: %d %s", rec.Code, rec.Body.String())
	}
	e.assertGone(a.id)
	if got := e.rowsOf(b.id); !maps.Equal(got, bRows) {
		t.Errorf("B's rows = %v, want %v", got, bRows)
	}
	rec = e.do(http.MethodGet, "/api/me", bTok, "")
	if m := decodeObject(t, rec); rec.Code != http.StatusOK || m["id"] != b.id.String() || m["display_name"] != "Bea" {
		t.Errorf("B's GET after A's delete: %d %s", rec.Code, rec.Body.String())
	}
	e.session(e.login(b.email, b.password))
}
