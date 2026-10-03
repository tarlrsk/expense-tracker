package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountlockoperatorsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/lockoperators"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send"
)

// Helpers of the PLAN-0002 T6 tests (/api/me, /api/admin/*, the operator command).

// defaultCategories is how many categories the users trigger seeds (migration 0003).
const defaultCategories = 13

// newEmail is an address no account has.
func newEmail() string { return "inv-" + uuid.NewString() + "@example.test" }

// cleanupEmail removes, when the test ends, the account the API or the operator command made
// for email, and its login attempts.
func (e *apiEnv) cleanupEmail(email string) {
	e.t.Cleanup(func() {
		e.exec("delete from users where email = $1::citext", email)
		e.exec("delete from login_attempts where email = $1::citext", email)
	})
}

// linkPattern is the whole link line of a set-password email (ADR-0068).
var linkPattern = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(testWebBaseURL) + `/set-password#token=([A-Za-z0-9_-]{43})$`)

// mailsTo returns the messages the API sent to address, oldest first.
func (e *apiEnv) mailsTo(address string) []send.Message {
	var out []send.Message
	for _, m := range e.mail.Sent() {
		if strings.EqualFold(m.To, address) {
			out = append(out, m)
		}
	}
	return out
}

// lastLink checks the latest email to address is a set-password email and returns its token.
func (e *apiEnv) lastLink(address string) string {
	e.t.Helper()
	msgs := e.mailsTo(address)
	if len(msgs) == 0 {
		e.t.Fatalf("no email to %s", address)
	}
	m := msgs[len(msgs)-1]
	if m.Subject != domain.SetPasswordSubject {
		e.t.Errorf("subject %q, want %q", m.Subject, domain.SetPasswordSubject)
	}
	for _, want := range []string{"valid for 7 days", "works once"} {
		if !strings.Contains(m.Text, want) {
			e.t.Errorf("email text lacks %q:\n%s", want, m.Text)
		}
	}
	match := linkPattern.FindStringSubmatch(m.Text)
	if match == nil {
		e.t.Fatalf("email has no %s/set-password#token=... line:\n%s", testWebBaseURL, m.Text)
	}
	return match[1]
}

func (e *apiEnv) invite(token, email string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/admin/invites", token, jsonBody(e.t, map[string]string{"email": email}))
}

func (e *apiEnv) sendLink(token, id string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/admin/users/"+id+"/set-password-link", token, "")
}

func (e *apiEnv) removeUser(token, id string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodDelete, "/api/admin/users/"+id, token, "")
}

func (e *apiEnv) deleteMe(token, password string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodDelete, "/api/me", token, jsonBody(e.t, map[string]string{"password": password}))
}

// userID returns the id of the account with email, failing the test when there is none.
func (e *apiEnv) userID(email string) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	if err := e.super.QueryRowContext(e.t.Context(), "select id from users where email = $1::citext", email).Scan(&id); err != nil {
		e.t.Fatalf("find the account: %v", err)
	}
	return id
}

func (e *apiEnv) role(id uuid.UUID) string {
	e.t.Helper()
	var role string
	if err := e.super.QueryRowContext(e.t.Context(), "select role from profiles where id = $1", id).Scan(&role); err != nil {
		e.t.Fatalf("read the role: %v", err)
	}
	return role
}

// userTables are the tables holding a user's rows, with the column naming the user.
var userTables = []struct{ table, column string }{
	{"users", "id"}, {"profiles", "id"}, {"sessions", "user_id"}, {"email_tokens", "user_id"}, {"categories", "owner_id"},
}

// rowsOf counts the user's rows in each of userTables.
func (e *apiEnv) rowsOf(id uuid.UUID) map[string]int {
	e.t.Helper()
	got := map[string]int{}
	for _, ut := range userTables {
		got[ut.table] = e.count("select count(*) from "+ut.table+" where "+ut.column+" = $1", id)
	}
	return got
}

// assertGone fails unless the user has no row left in any of userTables.
func (e *apiEnv) assertGone(id uuid.UUID) {
	e.t.Helper()
	for table, n := range e.rowsOf(id) {
		if n != 0 {
			e.t.Errorf("%s still has %d rows of %s", table, n, id)
		}
	}
}

// onlyOperators makes the given accounts the only operators the API can see: committed operators
// left over from earlier, interrupted runs are demoted. The tests of this package run one at a
// time, and other packages' tests work inside transactions they roll back, so nothing else is
// touched.
func (e *apiEnv) onlyOperators(keep ...uuid.UUID) {
	e.t.Helper()
	e.exec("update profiles set role = 'user' where role = 'operator' and not (id = any($1))", keep)
	if n := e.count("select count(*) from profiles where role = 'operator'"); n != len(keep) {
		e.t.Fatalf("operators = %d, want %d", n, len(keep))
	}
}

// holdOperatorLock takes the removals' advisory lock in a superuser transaction and returns the
// function that releases it.
func (e *apiEnv) holdOperatorLock() (release func()) {
	e.t.Helper()
	tx, err := e.super.BeginTx(e.t.Context(), nil)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := tx.ExecContext(e.t.Context(), "select pg_advisory_xact_lock($1)", accountlockoperatorsport.Key); err != nil {
		_ = tx.Rollback()
		e.t.Fatal(err)
	}
	release = func() { _ = tx.Rollback() }
	e.t.Cleanup(release)
	return release
}

// waitForLockWaiters waits until n transactions wait for the removals' advisory lock.
func (e *apiEnv) waitForLockWaiters(n int) {
	e.t.Helper()
	const q = `select count(*) from pg_locks where locktype = 'advisory' and not granted
	  and classid::bigint = 0 and objid::bigint = $1 and objsubid = 1`
	deadline := time.Now().Add(10 * time.Second)
	for {
		got := e.count(q, accountlockoperatorsport.Key)
		if got == n {
			return
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("lock waiters = %d, want %d", got, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// decodeObject decodes a JSON object response.
func decodeObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return m
}

// assertKeys fails unless m has exactly the keys want.
func assertKeys(t *testing.T, what string, m map[string]any, want ...string) {
	t.Helper()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Errorf("%s keys = %v, want exactly %v", what, keys, want)
	}
}

// parseTime parses a JSON time field.
func parseTime(t *testing.T, v any) time.Time {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("time field is %T %v, want a string", v, v)
	}
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return ts
}

// concurrently runs the functions at the same moment and returns their statuses sorted.
func concurrently(fns ...func() int) []int {
	codes := make([]int, len(fns))
	done := make(chan struct{})
	for i, fn := range fns {
		go func() {
			codes[i] = fn()
			done <- struct{}{}
		}()
	}
	for range fns {
		<-done
	}
	slices.Sort(codes)
	return codes
}
