package app

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Tests of the operator command (ADR-0035, ADR-0068). runOperator is the command without the
// process: the test database as app_login, the fake mailer and the real clock.

func (e *apiEnv) runOperator(email string) (string, error) {
	e.t.Helper()
	var out bytes.Buffer
	err := runOperator(e.t.Context(), e.deps, e.db, email, &out)
	return out.String(), err
}

const (
	createdLine  = "Created a new operator account; emailed it a new set-password link (valid for 7 days; earlier links no longer work).\n"
	relinkLine   = "The account is already an operator; emailed it a new set-password link (valid for 7 days; earlier links no longer work).\n"
	noopLine     = "The account is already an operator; it already has a password, so no email was sent.\n"
	promotedLine = "Made the existing account an operator; it already has a password, so no email was sent.\n"
)

func TestOperatorCommand(t *testing.T) {
	e := newAPIEnv(t)

	t.Run("creates an operator and emails a link; runs again without a second account", func(t *testing.T) {
		e := e.with(t)
		email := newEmail()
		e.cleanupEmail(email)

		out, err := e.runOperator("  " + email + " ")
		if err != nil || out != createdLine {
			t.Fatalf("first run: %q, %v", out, err)
		}
		id := e.userID(email)
		if e.role(id) != "operator" {
			t.Errorf("role = %q, want operator", e.role(id))
		}
		if n := e.count("select count(*) from categories where owner_id = $1", id); n != defaultCategories {
			t.Errorf("categories = %d, want %d", n, defaultCategories)
		}
		if n := len(e.mailsTo(email)); n != 1 {
			t.Fatalf("emails = %d, want 1", n)
		}
		first := e.lastLink(email)

		out, err = e.runOperator(strings.ToUpper(email))
		if err != nil || out != relinkLine {
			t.Fatalf("second run: %q, %v", out, err)
		}
		if n := e.count("select count(*) from users where email = $1::citext", email); n != 1 {
			t.Errorf("accounts = %d, want 1", n)
		}
		if n := len(e.mailsTo(email)); n != 2 {
			t.Fatalf("emails = %d, want 2", n)
		}
		second := e.lastLink(email)
		if rec := e.setPassword(first, newPassword); rec.Code != http.StatusBadRequest {
			t.Errorf("first link after the second run: %d, want 400", rec.Code)
		}
		s := e.session(e.setPassword(second, newPassword))
		if code, who := e.whoami(s.Token); code != http.StatusOK || who.UserID != id || who.Role != "operator" {
			t.Errorf("whoami: %d %+v", code, who)
		}

		out, err = e.runOperator(email)
		if err != nil || out != noopLine {
			t.Errorf("third run: %q, %v", out, err)
		}
		if n := len(e.mailsTo(email)); n != 2 {
			t.Errorf("emails = %d after the third run, want 2", n)
		}
		for _, secret := range []string{first, second} {
			if strings.Contains(e.logs.String(), secret) {
				t.Error("the logs contain a token")
			}
		}
	})

	t.Run("promotes a user who has a password, without email; their session keeps working", func(t *testing.T) {
		e := e.with(t)
		a := e.newAccount(accountOpts{})
		tok := e.freshSession(a.id)
		out, err := e.runOperator(a.email)
		if err != nil || out != promotedLine {
			t.Fatalf("run: %q, %v", out, err)
		}
		if n := len(e.mailsTo(a.email)); n != 0 {
			t.Errorf("emails = %d, want 0", n)
		}
		if code, who := e.whoami(tok); code != http.StatusOK || who.Role != "operator" {
			t.Errorf("whoami: %d %+v", code, who)
		}
	})

	t.Run("a failed email is an error that says to run again; running again sends it", func(t *testing.T) {
		e := e.with(t)
		email := newEmail()
		e.cleanupEmail(email)
		e.mail.FailWith(errors.New("smtp unreachable"))
		t.Cleanup(func() { e.mail.FailWith(nil) })

		out, err := e.runOperator(email)
		if err == nil || !strings.Contains(err.Error(), "run the command again") || out != "" {
			t.Fatalf("run with a failing mailer: %q, %v", out, err)
		}
		if strings.Contains(err.Error(), email) {
			t.Errorf("the error names the address: %v", err)
		}
		id := e.userID(email)
		if e.role(id) != "operator" {
			t.Errorf("role = %q, want operator", e.role(id))
		}

		e.mail.FailWith(nil)
		out, err = e.runOperator(email)
		if err != nil || out != relinkLine {
			t.Fatalf("run again: %q, %v", out, err)
		}
		e.session(e.setPassword(e.lastLink(email), newPassword))
	})

	t.Run("an email that is not one plain address creates nothing", func(t *testing.T) {
		e := e.with(t)
		for _, email := range []string{"Ann <ann-op@example.test>", "ann-op@example.test, bob-op@example.test", "plain"} {
			out, err := e.runOperator(email)
			if err == nil || !strings.Contains(err.Error(), domain.InviteEmailMessage) || out != "" {
				t.Errorf("run %q: %q, %v", email, out, err)
			}
		}
		if n := e.count("select count(*) from users where email::text like any(array['%ann-op@%', '%bob-op@%', 'plain'])"); n != 0 {
			t.Errorf("accounts created: %d", n)
		}
	})
}

// The arguments are checked before any setting is read or the database is opened.
func TestRunOperatorUsage(t *testing.T) {
	for _, args := range [][]string{{}, {"-email"}, {"-email", ""}, {"-email", "a@example.test", "extra"}, {"-unknown"}} {
		var stdout, stderr bytes.Buffer
		err := RunOperator(t.Context(), args, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), OperatorUsage) {
			t.Errorf("RunOperator(%q) = %v, want the usage", args, err)
		}
		if stdout.Len() != 0 {
			t.Errorf("RunOperator(%q) printed %q", args, stdout.String())
		}
	}
}
