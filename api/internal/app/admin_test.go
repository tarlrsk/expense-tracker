package app

import (
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// API-level tests of PLAN-0002 T6, operator side, on the Docker test database with the fake
// mailer (ADR-0035, ADR-0036, ADR-0037, ADR-0038, ADR-0068).

// The whole invite: the operator invites, the email carries the link, the link sets the
// password, and the new user logs in and has the default categories.
func TestInviteFlow(t *testing.T) {
	e := newAPIEnv(t)
	op := e.newAccount(accountOpts{operator: true})
	opTok := e.freshSession(op.id)
	email := newEmail()
	e.cleanupEmail(email)

	rec := e.invite(opTok, "  "+email+" ")
	if rec.Code != http.StatusCreated {
		t.Fatalf("invite: %d %s, want 201", rec.Code, rec.Body.String())
	}
	body := decodeObject(t, rec)
	assertKeys(t, "invite response", body, "user", "email_sent")
	if body["email_sent"] != true {
		t.Errorf("email_sent = %v, want true", body["email_sent"])
	}
	user, _ := body["user"].(map[string]any)
	assertKeys(t, "user", user, "id", "email", "display_name", "role", "status", "created_at", "last_active_at")
	if user["email"] != email || user["display_name"] != "" || user["role"] != "user" || user["status"] != "invited" ||
		user["last_active_at"] != nil {
		t.Errorf("user = %v", user)
	}
	if d := time.Since(parseTime(t, user["created_at"])); d < -time.Minute || d > time.Minute {
		t.Errorf("created_at %v is not about now", user["created_at"])
	}
	id := uuid.MustParse(user["id"].(string))
	if got := e.userID(email); got != id {
		t.Fatalf("stored account %s, response %s", got, id)
	}

	if n := len(e.mailsTo(email)); n != 1 {
		t.Fatalf("emails to the invitee = %d, want 1", n)
	}
	link := e.lastLink(email)
	if strings.Contains(rec.Body.String(), link) {
		t.Error("the invite response contains the token")
	}
	s := e.session(e.setPassword(link, newPassword))
	if code, who := e.whoami(s.Token); code != http.StatusOK || who.UserID != id || who.Role != "user" {
		t.Errorf("whoami after set password: %d %+v", code, who)
	}
	login := e.session(e.login(email, newPassword))
	if code, _ := e.whoami(login.Token); code != http.StatusOK {
		t.Errorf("whoami after login: %d", code)
	}
	if n := e.count("select count(*) from categories where owner_id = $1", id); n != defaultCategories {
		t.Errorf("categories = %d, want %d", n, defaultCategories)
	}
	if n := e.count("select count(*) from profiles where id = $1 and role = 'user' and display_name = ''", id); n != 1 {
		t.Errorf("profiles = %d, want 1 plain user profile", n)
	}
}

func TestInvite(t *testing.T) {
	e := newAPIEnv(t)
	op := e.newAccount(accountOpts{operator: true})
	opTok := e.freshSession(op.id)

	t.Run("an email that has an account is a conflict, whatever its case", func(t *testing.T) {
		e := e.with(t)
		existing := e.newAccount(accountOpts{})
		sent := len(e.mail.Sent())
		for _, email := range []string{existing.email, strings.ToUpper(existing.email), " " + existing.email + " "} {
			rec := e.invite(opTok, email)
			if rec.Code != http.StatusConflict || errorCode(t, rec) != "conflict" ||
				!strings.Contains(rec.Body.String(), domain.EmailTakenMessage) {
				t.Errorf("invite %q: %d %s, want 409 conflict", email, rec.Code, rec.Body.String())
			}
		}
		if n := e.count("select count(*) from users where email = $1::citext", existing.email); n != 1 {
			t.Errorf("accounts with the email = %d, want 1", n)
		}
		if n := len(e.mail.Sent()); n != sent {
			t.Errorf("emails sent: %d", n-sent)
		}
	})

	t.Run("two invites for one email at the same moment make one account", func(t *testing.T) {
		e := e.with(t)
		email := newEmail()
		e.cleanupEmail(email)
		codes := concurrently(
			func() int { return e.invite(opTok, email).Code },
			func() int { return e.invite(opTok, strings.ToUpper(email)).Code },
		)
		if !slices.Equal(codes, []int{http.StatusCreated, http.StatusConflict}) {
			t.Errorf("statuses = %v, want 201 and 409", codes)
		}
		if n := e.count("select count(*) from users where email = $1::citext", email); n != 1 {
			t.Errorf("accounts = %d, want 1", n)
		}
		if n := len(e.mailsTo(email)); n != 1 {
			t.Errorf("emails = %d, want 1", n)
		}
	})

	t.Run("malformed emails and bodies are invalid_input and create nothing", func(t *testing.T) {
		e := e.with(t)
		sent := len(e.mail.Sent())
		emails := []string{
			"", "   ", "plain", "@example.test", "ann@", "ann@@example.test", "Ann <ann-x@example.test>",
			"<ann-x@example.test>", "ann-x@example.test, bob-x@example.test", "ann-x@example.test bob-x@example.test",
			strings.Repeat("a", 243) + "@example.test",
		}
		for _, email := range emails {
			rec := e.invite(opTok, email)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("invite %q: %d %s, want 400 invalid_input", email, rec.Code, rec.Body.String())
			}
		}
		for name, body := range map[string]string{
			"no email":      `{}`,
			"not a string":  `{"email":5}`,
			"role field":    `{"email":"` + newEmail() + `","role":"operator"}`,
			"no body":       ``,
			"two addresses": `{"email":["a@example.test","b@example.test"]}`,
		} {
			rec := e.do(http.MethodPost, "/api/admin/invites", opTok, body)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_input" {
				t.Errorf("%s: %d %s, want 400 invalid_input", name, rec.Code, rec.Body.String())
			}
		}
		if n := e.count("select count(*) from users where email::text like any(array['%ann-x@%', '%bob-x@%'])"); n != 0 {
			t.Errorf("accounts created: %d", n)
		}
		if n := len(e.mail.Sent()); n != sent {
			t.Errorf("emails sent: %d", n-sent)
		}
	})

	t.Run("a failed email is still 201 with email_sent false; a new link fixes it", func(t *testing.T) {
		e := e.with(t)
		email := newEmail()
		e.cleanupEmail(email)
		e.mail.FailWith(errors.New("smtp unreachable"))
		t.Cleanup(func() { e.mail.FailWith(nil) })

		rec := e.invite(opTok, email)
		if rec.Code != http.StatusCreated {
			t.Fatalf("invite: %d %s, want 201", rec.Code, rec.Body.String())
		}
		body := decodeObject(t, rec)
		user, _ := body["user"].(map[string]any)
		if body["email_sent"] != false || user["email"] != email || user["status"] != "invited" {
			t.Errorf("body = %v", body)
		}
		id := e.userID(email)
		if n := e.count("select count(*) from email_tokens where user_id = $1", id); n != 1 {
			t.Errorf("links = %d, want 1", n)
		}
		if n := e.count("select count(*) from categories where owner_id = $1", id); n != defaultCategories {
			t.Errorf("categories = %d, want %d", n, defaultCategories)
		}
		logs := e.logs.String()
		if !strings.Contains(logs, `"msg":"set-password email not sent"`) || !strings.Contains(logs, id.String()) {
			t.Errorf("the failure is not logged:\n%s", logs)
		}
		if strings.Contains(logs, email) {
			t.Errorf("logs contain the address:\n%s", logs)
		}

		e.mail.FailWith(nil)
		rec = e.sendLink(opTok, id.String())
		if rec.Code != http.StatusOK || decodeObject(t, rec)["email_sent"] != true {
			t.Fatalf("send a new link: %d %s", rec.Code, rec.Body.String())
		}
		e.session(e.setPassword(e.lastLink(email), newPassword))
	})

	t.Run("the token and the link never reach a response or a log", func(t *testing.T) {
		e := e.with(t)
		email := newEmail()
		e.cleanupEmail(email)
		rec := e.invite(opTok, email)
		if rec.Code != http.StatusCreated {
			t.Fatalf("invite: %d", rec.Code)
		}
		token := e.lastLink(email)
		list := e.do(http.MethodGet, "/api/admin/users", opTok, "")
		resend := e.sendLink(opTok, e.userID(email).String())
		token2 := e.lastLink(email)
		for _, secret := range []string{token, token2, "set-password#token"} {
			for name, out := range map[string]string{
				"invite": rec.Body.String(), "list": list.Body.String(), "resend": resend.Body.String(), "logs": e.logs.String(),
			} {
				if strings.Contains(out, secret) {
					t.Errorf("%s contains %q", name, secret)
				}
			}
		}
		if strings.Contains(e.logs.String(), email) {
			t.Error("logs contain the invitee's address")
		}
	})
}

func TestSendLink(t *testing.T) {
	e := newAPIEnv(t)
	op := e.newAccount(accountOpts{operator: true})
	opTok := e.freshSession(op.id)

	t.Run("a new link cancels the old one; sessions last until the password is set", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		s1, s2 := e.freshSession(a.id), e.freshSession(a.id)
		old := e.newLink(a.id, time.Now().Add(domain.LinkLifetime), false)

		rec := e.sendLink(opTok, a.id.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("send link: %d %s", rec.Code, rec.Body.String())
		}
		body := decodeObject(t, rec)
		assertKeys(t, "send link response", body, "email_sent")
		if body["email_sent"] != true {
			t.Errorf("email_sent = %v", body["email_sent"])
		}
		link := e.lastLink(a.email)
		var expires time.Time
		if err := e.super.QueryRowContext(t.Context(), "select expires_at from email_tokens where user_id = $1", a.id).Scan(&expires); err != nil {
			t.Fatalf("read the link (want exactly one): %v", err)
		}
		if d := time.Until(expires) - domain.LinkLifetime; d < -time.Minute || d > time.Minute {
			t.Errorf("the link expires at %s, not about 7 days from now", expires)
		}
		for _, tok := range []string{s1, s2} {
			if code, _ := e.whoami(tok); code != http.StatusOK {
				t.Errorf("a session ended when the link was sent: %d", code)
			}
		}
		if rec := e.setPassword(old, newPassword); rec.Code != http.StatusBadRequest {
			t.Errorf("old link: %d, want 400", rec.Code)
		}
		e.session(e.setPassword(link, newPassword))
		for _, tok := range []string{s1, s2} {
			if code, _ := e.whoami(tok); code != http.StatusUnauthorized {
				t.Errorf("session after the new password: %d, want 401", code)
			}
		}
		e.session(e.login(a.email, newPassword))
	})

	t.Run("a failed email is 200 with email_sent false", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{noPassword: true})
		e.mail.FailWith(errors.New("smtp unreachable"))
		t.Cleanup(func() { e.mail.FailWith(nil) })
		rec := e.sendLink(opTok, a.id.String())
		if rec.Code != http.StatusOK || decodeObject(t, rec)["email_sent"] != false {
			t.Errorf("send link: %d %s, want 200 with email_sent false", rec.Code, rec.Body.String())
		}
		if n := e.count("select count(*) from email_tokens where user_id = $1", a.id); n != 1 {
			t.Errorf("links = %d, want 1", n)
		}
	})

	t.Run("an unknown or malformed id is 404", func(t *testing.T) {
		e := e.with(t)
		sent := len(e.mail.Sent())
		for _, id := range []string{uuid.NewString(), uuid.Nil.String(), "not-a-uuid", "123"} {
			rec := e.sendLink(opTok, id)
			if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
				t.Errorf("id %q: %d %s, want 404 not_found", id, rec.Code, rec.Body.String())
			}
		}
		if n := len(e.mail.Sent()); n != sent {
			t.Errorf("emails sent: %d", n-sent)
		}
	})
}

func TestListUsers(t *testing.T) {
	e := newAPIEnv(t)
	op := e.newAccount(accountOpts{operator: true})
	opTok := e.freshSession(op.id)
	now := time.Now().UTC().Truncate(time.Microsecond)

	active := e.newAccount(accountOpts{})
	invited := e.newAccount(accountOpts{noPassword: true})
	twin := e.newAccount(accountOpts{})
	e.exec("update users set created_at = $2 where id = $1", active.id, now.Add(-3*time.Hour))
	e.exec("update users set created_at = $2 where id = any($1)", []uuid.UUID{invited.id, twin.id}, now.Add(-2*time.Hour))
	e.exec("update profiles set display_name = 'Ann' where id = $1", active.id)
	e.newSession(active.id, now.Add(-90*time.Minute), now.Add(domain.SessionLifetime))
	e.newSession(active.id, now.Add(-30*time.Minute), now.Add(domain.SessionLifetime))

	rec := e.do(http.MethodGet, "/api/admin/users", opTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	body := decodeObject(t, rec)
	assertKeys(t, "list response", body, "users")
	items, _ := body["users"].([]any)
	byID := map[string]map[string]any{}
	var prevAt time.Time
	var prevID string
	for i, it := range items {
		item, _ := it.(map[string]any)
		assertKeys(t, "user", item, "id", "email", "display_name", "role", "status", "created_at", "last_active_at")
		id, _ := item["id"].(string)
		at := parseTime(t, item["created_at"])
		if i > 0 && (at.Before(prevAt) || (at.Equal(prevAt) && id <= prevID)) {
			t.Errorf("item %d (%s, %s) is not after (%s, %s)", i, at, id, prevAt, prevID)
		}
		prevAt, prevID = at, id
		byID[id] = item
	}

	for _, c := range []struct {
		name       string
		acc        account
		display    string
		role       string
		status     string
		lastActive *time.Time
	}{
		{"active user", active, "Ann", "user", "active", new(now.Add(-30 * time.Minute))},
		{"invited user", invited, "", "user", "invited", nil},
		{"user without a session", twin, "", "user", "active", nil},
	} {
		item := byID[c.acc.id.String()]
		if item == nil {
			t.Errorf("%s missing from the list", c.name)
			continue
		}
		if item["email"] != c.acc.email || item["display_name"] != c.display || item["role"] != c.role || item["status"] != c.status {
			t.Errorf("%s: %v", c.name, item)
		}
		switch {
		case c.lastActive == nil && item["last_active_at"] != nil:
			t.Errorf("%s: last_active_at = %v, want null", c.name, item["last_active_at"])
		case c.lastActive != nil && !parseTime(t, item["last_active_at"]).Equal(*c.lastActive):
			t.Errorf("%s: last_active_at = %v, want %s", c.name, item["last_active_at"], *c.lastActive)
		}
	}
	if item := byID[op.id.String()]; item == nil || item["role"] != "operator" || item["status"] != "active" || item["last_active_at"] == nil {
		t.Errorf("operator item = %v", item)
	}
	for _, secret := range []string{"argon2id", active.password, "password"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("the list contains %q", secret)
		}
	}
}

func TestRemoveUser(t *testing.T) {
	e := newAPIEnv(t)
	op := e.newAccount(accountOpts{operator: true})
	opTok := e.freshSession(op.id)

	t.Run("removes the user and every row of theirs, and nobody else's", func(t *testing.T) {
		e := e.with(t)
		b := e.newAccount(accountOpts{})
		bTok := e.freshSession(b.id)
		e.newLink(b.id, time.Now().Add(time.Hour), false)
		c := e.newAccount(accountOpts{})
		cTok := e.freshSession(c.id)
		want := map[string]int{"users": 1, "profiles": 1, "sessions": 1, "email_tokens": 1, "categories": defaultCategories}
		if got := e.rowsOf(b.id); !maps.Equal(got, want) {
			t.Fatalf("B's rows before = %v, want %v", got, want)
		}

		if rec := e.removeUser(opTok, b.id.String()); rec.Code != http.StatusNoContent {
			t.Fatalf("remove: %d %s", rec.Code, rec.Body.String())
		}
		e.assertGone(b.id)
		if code, _ := e.whoami(bTok); code != http.StatusUnauthorized {
			t.Errorf("B's token: %d, want 401", code)
		}
		wantC := map[string]int{"users": 1, "profiles": 1, "sessions": 1, "email_tokens": 0, "categories": defaultCategories}
		if got := e.rowsOf(c.id); !maps.Equal(got, wantC) {
			t.Errorf("C's rows = %v, want %v", got, wantC)
		}
		if code, who := e.whoami(cTok); code != http.StatusOK || who.UserID != c.id {
			t.Errorf("C's token: %d %+v", code, who)
		}
		if rec := e.removeUser(opTok, b.id.String()); rec.Code != http.StatusNotFound {
			t.Errorf("second removal: %d, want 404", rec.Code)
		}
	})

	t.Run("an operator cannot remove their own account here", func(t *testing.T) {
		e := e.with(t)
		rec := e.removeUser(opTok, op.id.String())
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), domain.RemoveSelfMessage) {
			t.Errorf("remove self: %d %s, want 409", rec.Code, rec.Body.String())
		}
		if n := e.count("select count(*) from users where id = $1", op.id); n != 1 {
			t.Error("the operator was removed")
		}
	})

	t.Run("another operator can be removed while one remains", func(t *testing.T) {
		e := e.with(t)
		other := e.newAccount(accountOpts{operator: true})
		if rec := e.removeUser(opTok, other.id.String()); rec.Code != http.StatusNoContent {
			t.Errorf("remove another operator: %d %s", rec.Code, rec.Body.String())
		}
		e.assertGone(other.id)
	})

	t.Run("an unknown or malformed id is 404", func(t *testing.T) {
		e := e.with(t)
		for _, id := range []string{uuid.NewString(), uuid.Nil.String(), "not-a-uuid", "123"} {
			rec := e.removeUser(opTok, id)
			if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
				t.Errorf("id %q: %d %s, want 404 not_found", id, rec.Code, rec.Body.String())
			}
		}
	})
}

// Every /api/admin route answers 401 without a token and 403 to a plain user, and does nothing
// for them (ADR-0019, ADR-0025).
func TestAdminRoutesOperatorOnly(t *testing.T) {
	e := newAPIEnv(t)
	user := e.newAccount(accountOpts{})
	userTok := e.freshSession(user.id)
	victim := e.newAccount(accountOpts{noPassword: true})
	email := newEmail()
	e.cleanupEmail(email)
	sent := len(e.mail.Sent())

	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/admin/users", ""},
		{http.MethodPost, "/api/admin/invites", jsonBody(t, map[string]string{"email": email})},
		{http.MethodPost, "/api/admin/users/" + victim.id.String() + "/set-password-link", ""},
		{http.MethodDelete, "/api/admin/users/" + victim.id.String(), ""},
	}
	for _, r := range routes {
		for _, c := range []struct {
			name, token string
			want        int
			code        string
		}{
			{"no token", "", http.StatusUnauthorized, "unauthenticated"},
			{"unknown token", domain.NewToken().Plain, http.StatusUnauthorized, "unauthenticated"},
			{"plain user", userTok, http.StatusForbidden, "forbidden"},
		} {
			rec := e.do(r.method, r.path, c.token, r.body)
			if rec.Code != c.want || errorCode(t, rec) != c.code {
				t.Errorf("%s %s, %s: %d %s, want %d %s", r.method, r.path, c.name, rec.Code, rec.Body.String(), c.want, c.code)
			}
			if strings.Contains(rec.Body.String(), victim.email) {
				t.Errorf("%s %s, %s: the answer names another user", r.method, r.path, c.name)
			}
		}
	}
	if n := e.count("select count(*) from users where id = $1", victim.id); n != 1 {
		t.Error("the victim was removed")
	}
	if n := e.count("select count(*) from email_tokens where user_id = $1", victim.id); n != 0 {
		t.Error("a link was made for the victim")
	}
	if n := e.count("select count(*) from users where email = $1::citext", email); n != 0 {
		t.Error("an account was invited")
	}
	if n := len(e.mail.Sent()); n != sent {
		t.Errorf("emails sent: %d", n-sent)
	}
}

// Two removals at the same moment cannot leave no operator (ADR-0038, ADR-0068): the test holds
// the removals' lock until both requests wait for it, then lets them go.
func TestLastOperatorRace(t *testing.T) {
	e := newAPIEnv(t)
	for _, c := range []struct {
		name string
		req  func(e *apiEnv, self, other account, selfTok string) int
	}{
		{"two operators remove each other", func(e *apiEnv, _, other account, tok string) int {
			return e.removeUser(tok, other.id.String()).Code
		}},
		{"two operators delete their own accounts", func(e *apiEnv, self, _ account, tok string) int {
			return e.deleteMe(tok, self.password).Code
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := e.with(t)
			a, b := e.newAccount(accountOpts{operator: true}), e.newAccount(accountOpts{operator: true})
			e.onlyOperators(a.id, b.id)
			aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)

			release := e.holdOperatorLock()
			results := make(chan []int, 1)
			go func() {
				results <- concurrently(
					func() int { return c.req(e, a, b, aTok) },
					func() int { return c.req(e, b, a, bTok) },
				)
			}()
			e.waitForLockWaiters(2)
			release()
			codes := <-results

			if !slices.Equal(codes, []int{http.StatusNoContent, http.StatusConflict}) {
				t.Errorf("statuses = %v, want one 204 and one 409", codes)
			}
			ids := []uuid.UUID{a.id, b.id}
			if n := e.count("select count(*) from profiles where id = any($1) and role = 'operator'", ids); n != 1 {
				t.Errorf("operators left = %d, want 1", n)
			}
			if n := e.count("select count(*) from users where id = any($1)", ids); n != 1 {
				t.Errorf("accounts left = %d, want 1", n)
			}
		})
	}
}
