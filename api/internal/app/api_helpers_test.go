package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send/sendtest"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// fixtureHasher hashes the passwords of test accounts with cheap parameters. Verification reads
// the parameters from the stored string, so the API checks them with these too.
var fixtureHasher = domain.NewHasher(domain.HashParams{Memory: 64, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32})

// apiEnv is the whole API on the Docker test database, logged in as app_login like the real
// one, plus a superuser connection for fixtures and checks. Each env has its own client address,
// so the per-address login limit of one test never touches another.
type apiEnv struct {
	t      *testing.T
	engine *gin.Engine
	super  *sql.DB
	ip     string
	logs   *syncBuffer
	// mail is the API's mailer; deps and db are its wiring, for tests that run a use case
	// without HTTP (the operator command).
	mail *sendtest.Fake
	deps registry.Deps
	db   *db.DB
}

// testWebBaseURL is WEB_BASE_URL in API tests: emailed links start with it.
const testWebBaseURL = "http://web.test:5173"

// syncBuffer is a bytes.Buffer safe for the concurrent requests of one test.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func newAPIEnv(t *testing.T) *apiEnv {
	t.Helper()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	database, err := db.Open(t.Context(), db.Config{
		URL: dbtest.LoginURL(t), StatementTimeout: 5 * time.Second, MaxOpenConns: 8, Logger: logger,
	})
	if err != nil {
		t.Fatalf("open the database as app_login: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	mailer := sendtest.New()
	deps := newDeps(config.Config{RequestTimeout: 20 * time.Second, WebBaseURL: testWebBaseURL}, logger, database, mailer)
	engine, err := newEngine(deps, database, testRoutes)
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	e := &apiEnv{t: t, engine: engine, super: dbtest.DB(t), ip: randomIP(t), logs: logs, mail: mailer, deps: deps, db: database}
	t.Cleanup(func() { e.exec("delete from login_attempts where ip = $1::inet", e.ip) })
	return e
}

// with returns a copy of e whose helpers report to t (a subtest) and register their cleanups
// there.
func (e *apiEnv) with(t *testing.T) *apiEnv {
	c := *e
	c.t = t
	return &c
}

// testRoutes adds routes that exist only in tests: who the caller is, and an operator route.
func testRoutes(r registry.Routes) {
	r.Authed.GET("/test/whoami", func(c *gin.Context) {
		caller, _ := httpx.CallerOf(c)
		c.JSON(http.StatusOK, whoami{UserID: caller.UserID, SessionID: caller.SessionID, Role: caller.Role})
	})
	r.Operator.GET("/test/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })
}

type whoami struct {
	UserID    uuid.UUID `json:"user_id"`
	SessionID uuid.UUID `json:"session_id"`
	Role      string    `json:"role"`
}

// randomIP returns an address in 10.0.0.0/8, different for each test.
func randomIP(t *testing.T) string {
	t.Helper()
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return fmt.Sprintf("10.%d.%d.%d", b[0], b[1], b[2])
}

// do sends a request from e.ip with an optional bearer token and JSON body.
func (e *apiEnv) do(method, path, token, body string) *httptest.ResponseRecorder {
	return e.doFrom(e.ip, method, path, token, body)
}

func (e *apiEnv) doFrom(ip, method, path, token, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	r := newJSONRequest(e.t, method, path, body)
	r.RemoteAddr = ip + ":40000"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return serveRequest(e.engine, r)
}

// newJSONRequest builds a request with an optional JSON body.
func newJSONRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	if body == "" {
		return httptest.NewRequestWithContext(context.Background(), method, path, nil)
	}
	r := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func serveRequest(engine *gin.Engine, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, r)
	return rec
}

func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func (e *apiEnv) login(email, password string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/auth/login", "", jsonBody(e.t, map[string]string{"email": email, "password": password}))
}

func (e *apiEnv) setPassword(link, password string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/auth/set-password", "", jsonBody(e.t, map[string]string{"token": link, "password": password}))
}

func (e *apiEnv) changePassword(token, current, next string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/me/password", token,
		jsonBody(e.t, map[string]string{"current_password": current, "new_password": next}))
}

// whoami returns the status of GET /api/test/whoami with token, and the caller on success.
func (e *apiEnv) whoami(token string) (int, whoami) {
	e.t.Helper()
	rec := e.do(http.MethodGet, "/api/test/whoami", token, "")
	var w whoami
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &w); err != nil {
			e.t.Fatalf("decode whoami %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, w
}

type sessionBody struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// session decodes a 200 login or set-password response, failing the test otherwise.
func (e *apiEnv) session(rec *httptest.ResponseRecorder) sessionBody {
	e.t.Helper()
	if rec.Code != http.StatusOK {
		e.t.Fatalf("status = %d, body %s; want 200", rec.Code, rec.Body.String())
	}
	dec := json.NewDecoder(rec.Body)
	dec.DisallowUnknownFields()
	var s sessionBody
	if err := dec.Decode(&s); err != nil {
		e.t.Fatalf("decode session body: %v", err)
	}
	if _, ok := domain.HashToken(s.Token); !ok {
		e.t.Fatalf("token %q is not a 32-byte base64url token", s.Token)
	}
	return s
}

// errorCode returns the error code of an error response.
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body httpx.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return body.Error.Code
}

func (e *apiEnv) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.super.ExecContext(context.WithoutCancel(e.t.Context()), q, args...); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
}

func (e *apiEnv) count(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.super.QueryRowContext(e.t.Context(), q, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
	return n
}

// account is a test user.
type account struct {
	id       uuid.UUID
	email    string
	password string
}

type accountOpts struct {
	noPassword bool // invite not accepted: password_hash = ''
	disabled   bool
	operator   bool
}

// newAccount inserts a user and profile as the superuser (the users trigger seeds its default
// categories), without going through the invite. The account,
// its sessions and links, and its login attempts are removed when the test ends.
func (e *apiEnv) newAccount(opts accountOpts) account {
	e.t.Helper()
	a := account{email: "u-" + uuid.NewString() + "@example.test", password: "pw-" + uuid.NewString()[:12]}
	hash := fixtureHasher.Hash(a.password)
	if opts.noPassword {
		hash = ""
	}
	var disabledAt *time.Time
	if opts.disabled {
		now := time.Now()
		disabledAt = &now
	}
	role := string(domain.RoleUser)
	if opts.operator {
		role = string(domain.RoleOperator)
	}
	if err := e.super.QueryRowContext(e.t.Context(),
		"insert into users (email, password_hash, disabled_at) values ($1, $2, $3) returning id",
		a.email, hash, disabledAt).Scan(&a.id); err != nil {
		e.t.Fatalf("insert user: %v", err)
	}
	e.exec("insert into profiles (id, role) values ($1, $2)", a.id, role)
	e.t.Cleanup(func() {
		e.exec("delete from users where id = $1", a.id)
		e.exec("delete from login_attempts where email = $1::citext", a.email)
	})
	return a
}

// newSession inserts a session for the user and returns its token.
func (e *apiEnv) newSession(userID uuid.UUID, lastUsed, expires time.Time) string {
	e.t.Helper()
	tok := domain.NewToken()
	e.exec("insert into sessions (user_id, token_hash, created_at, last_used_at, expires_at) values ($1, $2, $3, $3, $4)",
		userID, tok.Hash, lastUsed, expires)
	return tok.Plain
}

// freshSession is a session used just now that expires in 30 days.
func (e *apiEnv) freshSession(userID uuid.UUID) string {
	now := time.Now()
	return e.newSession(userID, now, now.Add(domain.SessionLifetime))
}

// newLink inserts a set_password link for the user and returns its token.
func (e *apiEnv) newLink(userID uuid.UUID, expires time.Time, used bool) string {
	e.t.Helper()
	tok := domain.NewToken()
	var usedAt *time.Time
	if used {
		now := time.Now()
		usedAt = &now
	}
	e.exec("insert into email_tokens (user_id, purpose, token_hash, expires_at, used_at) values ($1, 'set_password', $2, $3, $4)",
		userID, tok.Hash, expires, usedAt)
	return tok.Plain
}

// addAttempts inserts n failed attempts for email from ip at the given time.
func (e *apiEnv) addAttempts(n int, email, ip string, at time.Time) {
	e.t.Helper()
	for range n {
		e.exec("insert into login_attempts (email, ip, attempted_at) values ($1, $2::inet, $3)", email, ip, at)
	}
	e.t.Cleanup(func() { e.exec("delete from login_attempts where email = $1::citext", email) })
}

func (e *apiEnv) attemptsForEmail(email string) int {
	return e.count("select count(*) from login_attempts where email = $1::citext", email)
}

func (e *apiEnv) attemptsForIP(ip string) int {
	return e.count("select count(*) from login_attempts where ip = $1::inet", ip)
}

func (e *apiEnv) sessionCount(userID uuid.UUID) int {
	return e.count("select count(*) from sessions where user_id = $1", userID)
}
