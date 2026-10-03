package app

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// API-level tests of PLAN-0002 T5 on the Docker test database (ADR-0025, ADR-0037, ADR-0066).

const newPassword = "a new password, long enough"

func TestSetPassword(t *testing.T) {
	e := newAPIEnv(t)

	t.Run("valid link sets the password, logs in and ends earlier sessions", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{noPassword: true})
		old1, old2 := e.freshSession(a.id), e.freshSession(a.id)
		link := e.newLink(a.id, time.Now().Add(7*24*time.Hour), false)

		s := e.session(e.setPassword(link, newPassword))
		if d := time.Until(s.ExpiresAt); d < domain.SessionLifetime-time.Minute || d > domain.SessionLifetime+time.Minute {
			t.Errorf("expires_at %s is not about 30 days away", s.ExpiresAt)
		}
		if code, who := e.whoami(s.Token); code != http.StatusOK || who.UserID != a.id {
			t.Errorf("new session: status %d, user %s; want 200 for %s", code, who.UserID, a.id)
		}
		for _, old := range []string{old1, old2} {
			if code, _ := e.whoami(old); code != http.StatusUnauthorized {
				t.Errorf("earlier session still works: %d", code)
			}
		}
		if n := e.sessionCount(a.id); n != 1 {
			t.Errorf("sessions = %d, want 1", n)
		}
		e.session(e.login(a.email, newPassword))
		if rec := e.setPassword(link, "another long password"); rec.Code != http.StatusBadRequest {
			t.Errorf("second use: status %d, want 400", rec.Code)
		}
	})

	t.Run("every unusable link gets the identical answer", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{noPassword: true})
		disabled := e.newAccount(accountOpts{noPassword: true, disabled: true})
		links := map[string]string{
			"expired":   e.newLink(a.id, time.Now().Add(-time.Minute), false),
			"used":      e.newLink(a.id, time.Now().Add(time.Hour), true),
			"unknown":   domain.NewToken().Plain,
			"disabled":  e.newLink(disabled.id, time.Now().Add(time.Hour), false),
			"malformed": "not-a-token",
			"empty":     "",
		}
		var first string
		for name, link := range links {
			rec := e.setPassword(link, newPassword)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: status %d, want 400", name, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), domain.LinkInvalidMessage) {
				t.Errorf("%s: body %s", name, rec.Body.String())
			}
			if first == "" {
				first = rec.Body.String()
			} else if rec.Body.String() != first {
				t.Errorf("%s: body %s differs from %s", name, rec.Body.String(), first)
			}
		}
		if n := e.sessionCount(a.id) + e.sessionCount(disabled.id); n != 0 {
			t.Errorf("sessions created: %d", n)
		}
		if n := e.count("select count(*) from users where id = any($1) and password_hash <> ''", []uuid.UUID{a.id, disabled.id}); n != 0 {
			t.Errorf("passwords set: %d", n)
		}
	})

	t.Run("two concurrent uses give exactly one success", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{noPassword: true})
		link := e.newLink(a.id, time.Now().Add(time.Hour), false)
		const n = 4
		codes := make([]int, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() { codes[i] = e.setPassword(link, newPassword).Code })
		}
		wg.Wait()
		ok := 0
		for _, c := range codes {
			switch c {
			case http.StatusOK:
				ok++
			case http.StatusBadRequest:
			default:
				t.Errorf("unexpected status %d", c)
			}
		}
		if ok != 1 {
			t.Errorf("successes = %d (%v), want 1", ok, codes)
		}
		if n := e.sessionCount(a.id); n != 1 {
			t.Errorf("sessions = %d, want 1", n)
		}
	})

	t.Run("password rule is checked first and keeps the link", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{noPassword: true})
		link := e.newLink(a.id, time.Now().Add(time.Hour), false)
		for _, pw := range []string{"", "short", "123456789", strings.Repeat("x", 129)} {
			rec := e.setPassword(link, pw)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), domain.PasswordRuleMessage) {
				t.Errorf("password of %d chars: %d %s", len(pw), rec.Code, rec.Body.String())
			}
		}
		// 10 runes of 3 bytes each pass: the rule counts characters.
		e.session(e.setPassword(link, strings.Repeat("ข", 10)))
	})
}

func TestLoginOutcomes(t *testing.T) {
	e := newAPIEnv(t)
	good := e.newAccount(accountOpts{})

	t.Run("success", func(t *testing.T) {
		e := e.with(t)
		s := e.session(e.login("  "+strings.ToUpper(good.email)+" ", good.password))
		if code, who := e.whoami(s.Token); code != http.StatusOK || who.UserID != good.id || who.Role != "user" {
			t.Errorf("whoami = %d %+v", code, who)
		}
	})

	t.Run("failures are indistinguishable and each is counted", func(t *testing.T) {
		e := e.with(t)
		unaccepted := e.newAccount(accountOpts{noPassword: true})
		disabled := e.newAccount(accountOpts{disabled: true})
		unknown := "nobody-" + uuid.NewString() + "@example.test"
		t.Cleanup(func() { e.exec("delete from login_attempts where email = $1::citext", unknown) })
		cases := []struct{ name, email, password string }{
			{"unknown email", unknown, "whatever password"},
			{"wrong password", good.email, "wrong password!"},
			{"invite not accepted", unaccepted.email, ""},
			{"disabled account", disabled.email, disabled.password},
		}
		var first string
		for _, c := range cases {
			rec := e.login(c.email, c.password)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s: status %d, want 401", c.name, rec.Code)
			}
			if first == "" {
				first = rec.Body.String()
			} else if rec.Body.String() != first {
				t.Errorf("%s: body %s, want the same as %s", c.name, rec.Body.String(), first)
			}
			if n := e.attemptsForEmail(c.email); n != 1 {
				t.Errorf("%s: %d attempts recorded, want 1", c.name, n)
			}
		}
		if !strings.Contains(first, domain.LoginFailedMessage) || errorCode(t, e.login(unknown, "x")) != "unauthenticated" {
			t.Errorf("body %s", first)
		}
	})

	t.Run("bad input", func(t *testing.T) {
		e := e.with(t)
		for name, body := range map[string]string{
			"empty email":     `{"email":"   ","password":"x"}`,
			"long email":      jsonBody(t, map[string]string{"email": strings.Repeat("a", 250) + "@x.io", "password": "x"}),
			"unknown field":   `{"email":"a@example.test","password":"x","admin":true}`,
			"malformed":       `{"email":`,
			"wrong type":      `{"email":1,"password":"x"}`,
			"two values":      `{"email":"a@example.test","password":"x"}{}`,
			"no body":         ``,
			"too large":       `{"email":"a@example.test","password":"` + strings.Repeat("x", 70<<10) + `"}`,
			"not json object": `"a@example.test"`,
		} {
			rec := e.do(http.MethodPost, "/api/auth/login", "", body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("%s: %d %s, want 400 invalid_input", name, rec.Code, rec.Body.String())
			}
		}
	})
}

func TestLoginRateLimit(t *testing.T) {
	t.Run("6th attempt for an email is refused even with the right password", func(t *testing.T) {
		e := newAPIEnv(t)
		a := e.newAccount(accountOpts{})
		for i := range domain.MaxFailuresPerEmail {
			// From different addresses: the email's count does not depend on the address.
			ip := randomIP(t)
			t.Cleanup(func() { e.exec("delete from login_attempts where ip = $1::inet", ip) })
			if rec := e.doFrom(ip, http.MethodPost, "/api/auth/login", "",
				jsonBody(t, map[string]string{"email": a.email, "password": "wrong"})); rec.Code != http.StatusUnauthorized {
				t.Fatalf("failure %d: status %d", i+1, rec.Code)
			}
		}
		rec := e.login(a.email, a.password)
		if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "rate_limited" {
			t.Fatalf("6th attempt: %d %s, want 429", rec.Code, rec.Body.String())
		}
		if n := e.attemptsForEmail(a.email); n != domain.MaxFailuresPerEmail {
			t.Errorf("attempts = %d, want %d (a refused attempt is not recorded)", n, domain.MaxFailuresPerEmail)
		}
		if n := e.sessionCount(a.id); n != 0 {
			t.Errorf("sessions = %d, want 0", n)
		}
	})

	t.Run("20 failures from one address block the 21st, whatever the email or proxy header", func(t *testing.T) {
		e := newAPIEnv(t)
		a := e.newAccount(accountOpts{})
		for i := range domain.MaxFailuresPerIP {
			email := "x" + uuid.NewString() + "@example.test"
			req := map[string]string{"email": email, "password": "wrong"}
			if rec := e.do(http.MethodPost, "/api/auth/login", "", jsonBody(t, req)); rec.Code != http.StatusUnauthorized {
				t.Fatalf("failure %d: status %d", i+1, rec.Code)
			}
		}
		rec := e.login(a.email, a.password)
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("21st attempt: %d %s, want 429", rec.Code, rec.Body.String())
		}
		// A spoofed X-Forwarded-For does not change the address: no proxy is trusted.
		spoofed := e.loginWithHeader(a.email, a.password, "X-Forwarded-For", "192.0.2.77")
		if spoofed != http.StatusTooManyRequests {
			t.Errorf("with X-Forwarded-For: status %d, want 429", spoofed)
		}
		// Another address is not blocked.
		other := randomIP(t)
		t.Cleanup(func() { e.exec("delete from login_attempts where ip = $1::inet", other) })
		if rec := e.doFrom(other, http.MethodPost, "/api/auth/login", "",
			jsonBody(t, map[string]string{"email": a.email, "password": a.password})); rec.Code != http.StatusOK {
			t.Errorf("from another address: %d, want 200", rec.Code)
		}
	})

	t.Run("attempts older than 15 minutes do not count", func(t *testing.T) {
		e := newAPIEnv(t)
		a := e.newAccount(accountOpts{})
		e.addAttempts(domain.MaxFailuresPerIP, a.email, e.ip, time.Now().Add(-16*time.Minute))
		e.addAttempts(domain.MaxFailuresPerEmail-1, a.email, e.ip, time.Now().Add(-14*time.Minute))
		e.session(e.login(a.email, a.password))
	})

	t.Run("success clears the email's attempts but not the address's", func(t *testing.T) {
		e := newAPIEnv(t)
		a := e.newAccount(accountOpts{})
		b := e.newAccount(accountOpts{})
		other := randomIP(t)
		t.Cleanup(func() { e.exec("delete from login_attempts where ip = $1::inet", other) })
		e.login(a.email, "wrong one")
		e.login(a.email, "wrong two")
		e.login(b.email, "wrong three")
		e.addAttempts(1, a.email, other, time.Now())
		e.session(e.login(a.email, a.password))
		if n := e.attemptsForEmail(a.email); n != 0 {
			t.Errorf("attempts for the email = %d, want 0", n)
		}
		if n := e.attemptsForIP(e.ip); n != 1 {
			t.Errorf("attempts for the address = %d, want 1 (the other email's)", n)
		}
		if n := e.attemptsForEmail(b.email); n != 1 {
			t.Errorf("attempts for the other email = %d, want 1", n)
		}
	})

	t.Run("a failed login removes attempts older than 24 hours", func(t *testing.T) {
		e := newAPIEnv(t)
		oldEmail, recentEmail := "old-"+uuid.NewString()+"@example.test", "recent-"+uuid.NewString()+"@example.test"
		e.addAttempts(2, oldEmail, randomIP(t), time.Now().Add(-25*time.Hour))
		e.addAttempts(1, recentEmail, randomIP(t), time.Now().Add(-23*time.Hour))
		a := e.newAccount(accountOpts{})
		e.login(a.email, "wrong")
		if n := e.attemptsForEmail(oldEmail); n != 0 {
			t.Errorf("attempts older than a day = %d, want 0", n)
		}
		if n := e.attemptsForEmail(recentEmail); n != 1 {
			t.Errorf("attempts younger than a day = %d, want 1", n)
		}
	})
}

// loginWithHeader logs in with one extra request header and returns the status.
func (e *apiEnv) loginWithHeader(email, password, header, value string) int {
	e.t.Helper()
	r := newJSONRequest(e.t, http.MethodPost, "/api/auth/login", jsonBody(e.t, map[string]string{"email": email, "password": password}))
	r.RemoteAddr = e.ip + ":40000"
	r.Header.Set(header, value)
	return serveRequest(e.engine, r).Code
}

func TestSessionCheck(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	disabled := e.newAccount(accountOpts{disabled: true})
	now := time.Now()

	t.Run("unusable tokens are all the same 401", func(t *testing.T) {
		e := e.with(t)
		valid := e.freshSession(a.id)
		expired := e.newSession(a.id, now.Add(-31*24*time.Hour), now.Add(-time.Hour))
		disabledTok := e.freshSession(disabled.id)
		headers := map[string]string{
			"missing":         "",
			"other scheme":    "Basic " + valid,
			"no token":        "Bearer",
			"empty token":     "Bearer ",
			"two parts":       "Bearer " + valid + " x",
			"malformed token": "Bearer abc",
			"unknown token":   "Bearer " + domain.NewToken().Plain,
			"expired":         "Bearer " + expired,
			"disabled user":   "Bearer " + disabledTok,
		}
		var first string
		for name, h := range headers {
			r := newJSONRequest(t, http.MethodGet, "/api/test/whoami", "")
			if h != "" {
				r.Header.Set("Authorization", h)
			}
			rec := serveRequest(e.engine, r)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s: status %d, want 401", name, rec.Code)
			}
			if first == "" {
				first = rec.Body.String()
			} else if rec.Body.String() != first {
				t.Errorf("%s: body %s differs from %s", name, rec.Body.String(), first)
			}
		}
		if !strings.Contains(first, `"code":"unauthenticated"`) {
			t.Errorf("body %s", first)
		}
		// The scheme is case-insensitive.
		r := newJSONRequest(t, http.MethodGet, "/api/test/whoami", "")
		r.Header.Set("Authorization", "bearer "+valid)
		if rec := serveRequest(e.engine, r); rec.Code != http.StatusOK {
			t.Errorf("lower-case scheme: %d", rec.Code)
		}
	})

	t.Run("last use is written at most once an hour", func(t *testing.T) {
		e := e.with(t)
		stale := now.Add(-2 * time.Hour)
		recent := now.Add(-30 * time.Minute)
		staleTok := e.newSession(a.id, stale, stale.Add(domain.SessionLifetime))
		recentTok := e.newSession(a.id, recent, recent.Add(domain.SessionLifetime))
		for _, tok := range []string{staleTok, recentTok} {
			if code, _ := e.whoami(tok); code != http.StatusOK {
				t.Fatalf("whoami: %d", code)
			}
		}
		read := func(tok string) (lastUsed, expires time.Time) {
			hash, _ := domain.HashToken(tok)
			if err := e.super.QueryRowContext(t.Context(),
				"select last_used_at, expires_at from sessions where token_hash = $1", hash).Scan(&lastUsed, &expires); err != nil {
				t.Fatal(err)
			}
			return lastUsed, expires
		}
		lu, ex := read(staleTok)
		if time.Since(lu) > time.Minute || ex.Sub(lu) != domain.SessionLifetime {
			t.Errorf("stale session: last_used_at %s, expires_at %s; want now and now + 30 days", lu, ex)
		}
		lu, ex = read(recentTok)
		if !lu.Equal(recent.Truncate(time.Microsecond)) || !ex.Equal(recent.Add(domain.SessionLifetime).Truncate(time.Microsecond)) {
			t.Errorf("recent session moved: last_used_at %s (was %s), expires_at %s", lu, recent, ex)
		}
	})

	t.Run("logout ends only the current session", func(t *testing.T) {
		e := e.with(t)
		tok, other := e.freshSession(a.id), e.freshSession(a.id)
		if rec := e.do(http.MethodPost, "/api/auth/logout", tok, ""); rec.Code != http.StatusNoContent {
			t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
		}
		if code, _ := e.whoami(tok); code != http.StatusUnauthorized {
			t.Errorf("logged-out token: %d, want 401", code)
		}
		if code, _ := e.whoami(other); code != http.StatusOK {
			t.Errorf("other session: %d, want 200", code)
		}
		if rec := e.do(http.MethodPost, "/api/auth/logout", tok, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("second logout: %d, want 401", rec.Code)
		}
		if rec := e.do(http.MethodPost, "/api/auth/logout", "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("logout without a token: %d, want 401", rec.Code)
		}
	})
}

func TestChangePassword(t *testing.T) {
	t.Run("success keeps this session, ends the others and swaps the password", func(t *testing.T) {
		e := newAPIEnv(t)
		a := e.newAccount(accountOpts{})
		tok, other := e.freshSession(a.id), e.freshSession(a.id)
		e.login(a.email, "one earlier failure")
		if rec := e.changePassword(tok, a.password, newPassword); rec.Code != http.StatusNoContent {
			t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
		}
		if code, _ := e.whoami(tok); code != http.StatusOK {
			t.Errorf("current session: %d, want 200", code)
		}
		if code, _ := e.whoami(other); code != http.StatusUnauthorized {
			t.Errorf("other session: %d, want 401", code)
		}
		if n := e.attemptsForEmail(a.email); n != 1 {
			t.Errorf("attempts = %d, want 1 (the success is not counted)", n)
		}
		if rec := e.login(a.email, a.password); rec.Code != http.StatusUnauthorized {
			t.Errorf("old password: %d, want 401", rec.Code)
		}
		e.session(e.login(a.email, newPassword))
	})

	t.Run("wrong current password is 400, counted, and limited", func(t *testing.T) {
		e := newAPIEnv(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		e.login(a.email, "a failed login counts too")
		for i := 1; i < domain.MaxFailuresPerEmail; i++ {
			rec := e.changePassword(tok, "not the password", newPassword)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), domain.WrongCurrentPasswordMessage) {
				t.Fatalf("wrong current %d: %d %s", i, rec.Code, rec.Body.String())
			}
		}
		if n := e.attemptsForEmail(a.email); n != domain.MaxFailuresPerEmail {
			t.Errorf("attempts = %d, want %d", n, domain.MaxFailuresPerEmail)
		}
		rec := e.changePassword(tok, a.password, newPassword)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("at the limit: %d %s, want 429", rec.Code, rec.Body.String())
		}
		if rec := e.login(a.email, a.password); rec.Code != http.StatusTooManyRequests {
			t.Errorf("login at the limit: %d, want 429", rec.Code)
		}
		if code, _ := e.whoami(tok); code != http.StatusOK {
			t.Errorf("session after failures: %d, want 200 (not logged out)", code)
		}
	})

	t.Run("new password rule and bad input", func(t *testing.T) {
		e := newAPIEnv(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		if rec := e.changePassword(tok, a.password, "short"); rec.Code != http.StatusBadRequest ||
			!strings.Contains(rec.Body.String(), domain.PasswordRuleMessage) {
			t.Errorf("short new password: %d %s", rec.Code, rec.Body.String())
		}
		if n := e.attemptsForEmail(a.email); n != 0 {
			t.Errorf("attempts = %d, want 0 (rule checked before anything else)", n)
		}
		rec := e.do(http.MethodPost, "/api/me/password", tok, `{"current_password":"x","new_password":"y","user_id":"z"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("unknown field: %d", rec.Code)
		}
		if rec := e.changePassword("", a.password, newPassword); rec.Code != http.StatusUnauthorized {
			t.Errorf("no token: %d, want 401", rec.Code)
		}
	})
}

// Every endpoint acts only for the caller (ADR-0014): A's logout and password change leave B's
// sessions and password alone.
func TestAccountCrossUser(t *testing.T) {
	e := newAPIEnv(t)
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok := e.freshSession(a.id)
	bTok1, bTok2 := e.freshSession(b.id), e.freshSession(b.id)

	if code, who := e.whoami(aTok); code != http.StatusOK || who.UserID != a.id {
		t.Fatalf("A's token: %d %+v", code, who)
	}
	if rec := e.changePassword(aTok, a.password, newPassword); rec.Code != http.StatusNoContent {
		t.Fatalf("A changes password: %d %s", rec.Code, rec.Body.String())
	}
	// B's password cannot be changed with A's token, even knowing it.
	if rec := e.changePassword(aTok, b.password, "something else entirely"); rec.Code != http.StatusBadRequest {
		t.Errorf("A's token with B's password: %d, want 400", rec.Code)
	}
	if rec := e.do(http.MethodPost, "/api/auth/logout", aTok, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("A logs out: %d", rec.Code)
	}
	for _, tok := range []string{bTok1, bTok2} {
		if code, who := e.whoami(tok); code != http.StatusOK || who.UserID != b.id {
			t.Errorf("B's session after A's changes: %d %+v", code, who)
		}
	}
	if n := e.sessionCount(b.id); n != 2 {
		t.Errorf("B's sessions = %d, want 2", n)
	}
	e.session(e.login(b.email, b.password))
	if rec := e.login(b.email, newPassword); rec.Code != http.StatusUnauthorized {
		t.Errorf("B with A's new password: %d, want 401", rec.Code)
	}
	// A's set-password link sets only A's password.
	c := e.newAccount(accountOpts{noPassword: true})
	s := e.session(e.setPassword(e.newLink(c.id, time.Now().Add(time.Hour), false), newPassword))
	if code, who := e.whoami(s.Token); code != http.StatusOK || who.UserID != c.id {
		t.Errorf("link session: %d %+v, want %s", code, who, c.id)
	}
	e.session(e.login(b.email, b.password))
}

func TestOperatorGuard(t *testing.T) {
	e := newAPIEnv(t)
	user := e.newAccount(accountOpts{})
	op := e.newAccount(accountOpts{operator: true})
	userTok, opTok := e.freshSession(user.id), e.freshSession(op.id)

	cases := []struct {
		name  string
		token string
		want  int
		code  string
	}{
		{"no token", "", http.StatusUnauthorized, "unauthenticated"},
		{"user", userTok, http.StatusForbidden, "forbidden"},
		{"operator", opTok, http.StatusNoContent, ""},
	}
	for _, c := range cases {
		rec := e.do(http.MethodGet, "/api/admin/test/ping", c.token, "")
		if rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
		}
		if c.code != "" && errorCode(t, rec) != c.code {
			t.Errorf("%s: code %s, want %s", c.name, errorCode(t, rec), c.code)
		}
	}
	// The role is read on every request: a change takes effect at once.
	e.exec("update profiles set role = 'operator' where id = $1", user.id)
	if rec := e.do(http.MethodGet, "/api/admin/test/ping", userTok, ""); rec.Code != http.StatusNoContent {
		t.Errorf("after promotion: %d, want 204", rec.Code)
	}
	e.exec("update profiles set role = 'user' where id = $1", op.id)
	if rec := e.do(http.MethodGet, "/api/admin/test/ping", opTok, ""); rec.Code != http.StatusForbidden {
		t.Errorf("after demotion: %d, want 403", rec.Code)
	}
}

// No password, token or email reaches the logs, for a failed and a successful login and for the
// other account endpoints; the request line carries only the error kind and message.
func TestAccountLogsHoldNoSecrets(t *testing.T) {
	e := newAPIEnv(t)
	a := e.newAccount(accountOpts{})
	link := e.newLink(e.newAccount(accountOpts{noPassword: true}).id, time.Now().Add(time.Hour), false)

	e.login(a.email, "wrong-secret-password")
	s := e.session(e.login(a.email, a.password))
	rec := e.changePassword(s.Token, "wrong-current-secret", newPassword)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("change: %d", rec.Code)
	}
	set := e.session(e.setPassword(link, "link-password-secret"))
	e.do(http.MethodPost, "/api/auth/logout", s.Token, "")

	logs := e.logs.String()
	for _, secret := range []string{
		a.email, strings.Split(a.email, "@")[0], a.password, "wrong-secret-password", "wrong-current-secret",
		"link-password-secret", s.Token, set.Token, link,
	} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain %q:\n%s", secret, logs)
		}
	}
	for _, want := range []string{
		`"route":"/api/auth/login","path":"/api/auth/login","status":401`,
		`"error":"unauthenticated: ` + domain.LoginFailedMessage + `"`,
		`"error":"invalid_input: ` + domain.WrongCurrentPasswordMessage + `"`,
		`"route":"/api/auth/set-password","path":"/api/auth/set-password","status":200`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %s:\n%s", want, logs)
		}
	}
}
