package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
)

func TestRun(t *testing.T) {
	const secret = "s3cret-pw"
	tests := []struct {
		name     string
		args     []string
		url      string
		wantCode int
		wantErr  string
	}{
		{name: "no command", args: nil, wantCode: 2, wantErr: "usage"},
		{name: "unknown command", args: []string{"sideways"}, wantCode: 2, wantErr: "unknown command"},
		{name: "two commands", args: []string{"up", "down"}, wantCode: 2, wantErr: "usage"},
		{name: "empty MIGRATION_DATABASE_URL", args: []string{"status"}, url: "", wantCode: 1, wantErr: "MIGRATION_DATABASE_URL is empty"},
		{name: "empty for login-password", args: []string{"login-password"}, url: "", wantCode: 1, wantErr: "MIGRATION_DATABASE_URL is empty"},
		{name: "invalid MIGRATION_DATABASE_URL", args: []string{"up"}, url: "postgres://u:" + secret + "@[bad", wantCode: 1, wantErr: "not a valid"},
		{
			name: "neon pooler host", args: []string{"up"},
			url:      "postgres://owner:" + secret + "@ep-cool-name-123456-pooler.ap-southeast-1.aws.neon.tech/neondb?sslmode=require",
			wantCode: 1, wantErr: "connection pooler",
		},
		{
			name: "pooler for login-password", args: []string{"login-password"},
			url:      "postgres://owner:" + secret + "@ep-cool-name-123456-pooler.ap-southeast-1.aws.neon.tech/neondb?sslmode=require",
			wantCode: 1, wantErr: "connection pooler",
		},
		{
			name: "pooler among several hosts", args: []string{"status"},
			url:      "postgres://owner:" + secret + "@ep-a.aws.neon.tech,ep-a-POOLER.aws.neon.tech/neondb",
			wantCode: 1, wantErr: "connection pooler",
		},
		{
			name: "login-password needs a URL", args: []string{"login-password"},
			url:      "host=127.0.0.1 port=1 user=u password=" + secret + " dbname=x",
			wantCode: 1, wantErr: "URL form",
		},
		{
			// Port 1 refuses at once; the error must not carry the password.
			name: "unreachable database", args: []string{"status"},
			url:      "postgres://u:" + secret + "@127.0.0.1:1/x?sslmode=disable&connect_timeout=5",
			wantCode: 1, wantErr: "connect",
		},
		{
			name: "unreachable for login-password", args: []string{"login-password"},
			url:      "postgres://u:" + secret + "@127.0.0.1:1/x?sslmode=disable&connect_timeout=5",
			wantCode: 1, wantErr: "connect",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(name string) string {
				if name == "MIGRATION_DATABASE_URL" {
					return tt.url
				}
				t.Errorf("read setting %s; only MIGRATION_DATABASE_URL is allowed", name)
				return ""
			}
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), tt.args, getenv, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantErr)
			}
			if out := stdout.String() + stderr.String(); strings.Contains(out, secret) {
				t.Errorf("output leaks the password: %q", out)
			}
		})
	}
}

func TestScrub(t *testing.T) {
	const password = "p@ss"
	dsn := "postgres://u:" + "p%40ss" + "@h/db"
	got := scrub("failed for "+dsn+" with "+password, dsn, password)
	if strings.Contains(got, password) || strings.Contains(got, dsn) {
		t.Fatalf("scrub left a secret: %q", got)
	}
}

func TestLoginURLBase(t *testing.T) {
	// The owner's name and password are cut from every case's result.
	const owner = "owner:pw@"
	tests := []struct {
		name, dsn, database, want string
	}{
		{
			name: "neon", dsn: "postgres://" + owner + "ep-x.aws.neon.tech/neondb?sslmode=require&channel_binding=require",
			want: "postgres://ep-x.aws.neon.tech/neondb?sslmode=require&channel_binding=require",
		},
		{
			name: "port kept", dsn: "postgresql://" + owner + "127.0.0.1:5433/expense_test?sslmode=disable",
			want: "postgresql://127.0.0.1:5433/expense_test?sslmode=disable",
		},
		{name: "missing database filled in", dsn: "postgres://" + owner + "h", database: "neondb", want: "postgres://h/neondb"},
		{name: "owner in query parameters dropped", dsn: "postgres://h/db?user=owner&password=pw&sslmode=require", want: "postgres://h/db?sslmode=require"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := loginURLBase(tt.dsn, tt.database)
			if err != nil {
				t.Fatal(err)
			}
			if got := u.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// loginFixture is a throw-away owner and login role on the test database, as migration 0001
// makes them on Neon: the owner creates the login role, so it may change its password.
type loginFixture struct {
	role, ownerPassword string
	getenv              func(string) string
}

func newLoginFixture(t *testing.T) loginFixture {
	t.Helper()
	shared := dbtest.DB(t)
	cfg := dbtest.Config(t)
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	owner, role := "t"+hex.EncodeToString(suffix)+"_owner", "t"+hex.EncodeToString(suffix)+"_login"
	ownerPassword := "owner-" + hex.EncodeToString(suffix) + "-pw"

	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		for _, r := range []string{role, owner} {
			if _, err := shared.ExecContext(ctx, "drop role if exists "+pgx.Identifier{r}.Sanitize()); err != nil {
				t.Errorf("drop role %s: %v", r, err)
			}
		}
	})
	if _, err := shared.ExecContext(t.Context(), "create role "+pgx.Identifier{owner}.Sanitize()+
		" login createrole password '"+ownerPassword+"'"); err != nil {
		t.Fatal(err)
	}
	ownerURL := url.URL{
		Scheme: "postgres", User: url.UserPassword(owner, ownerPassword),
		Host: cfg.Host + ":" + strconv.Itoa(int(cfg.Port)), Path: "/" + cfg.Database, RawQuery: "sslmode=disable",
	}
	ownerCfg := cfg.Copy()
	ownerCfg.User, ownerCfg.Password = owner, ownerPassword
	ownerConn, err := pgx.ConnectConfig(t.Context(), ownerCfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ownerConn.Exec(t.Context(), "create role "+pgx.Identifier{role}.Sanitize()+" login noinherit")
	_ = ownerConn.Close(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return loginFixture{role: role, ownerPassword: ownerPassword, getenv: func(name string) string {
		if name != "MIGRATION_DATABASE_URL" {
			t.Errorf("read setting %s", name)
			return ""
		}
		return ownerURL.String()
	}}
}

// run runs login-password with steps and checks what must hold whatever the outcome: the
// owner's password never appears, and stdout is empty or exactly one DATABASE_URL='...' line.
// It returns the exit code, the URL from the line ("" when none) and stderr.
func (f loginFixture) run(t *testing.T, steps passwordSteps) (code int, line, stderrText string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code = runWith(t.Context(), []string{"login-password"}, f.getenv, &stdout, &stderr, f.role, steps)
	if out := stdout.String() + stderr.String(); strings.Contains(out, f.ownerPassword) {
		t.Errorf("output contains the owner's password: %q", out)
	}
	if stdout.Len() == 0 {
		return code, "", stderr.String()
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout has %d lines, want the one DATABASE_URL line: %q", len(lines), stdout.String())
	}
	line, ok := strings.CutPrefix(lines[0], "DATABASE_URL='")
	line, ok2 := strings.CutSuffix(line, "'")
	if !ok || !ok2 {
		t.Fatalf("line %q is not DATABASE_URL='...'", lines[0])
	}
	if pw, _ := url.Parse(line); pw != nil {
		if p, _ := pw.User.Password(); p != "" && strings.Contains(stderr.String(), p) {
			t.Errorf("stderr contains the new password: %q", stderr.String())
		}
	}
	return code, line, stderr.String()
}

// connectAs logs in with connURL and checks the role; it returns the login error.
func connectAs(t *testing.T, connURL, role string) error {
	t.Helper()
	c, err := pgx.Connect(t.Context(), connURL)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close(t.Context()) }()
	var who string
	if err := c.QueryRow(t.Context(), "select current_user").Scan(&who); err != nil {
		return err
	}
	if who != role {
		t.Errorf("logged in as %s, want %s", who, role)
	}
	return nil
}

const fallbackNotice = "did not accept a pre-hashed password"

// The normal path: the verifier is accepted, the line is checked and printed, it logs in, and
// a second run makes the first password useless.
func TestLoginPassword(t *testing.T) {
	f := newLoginFixture(t)

	code, first, stderr := f.run(t, defaultPasswordSteps)
	if code != 0 || first == "" {
		t.Fatalf("exit code %d, line %q: %s", code, first, stderr)
	}
	if !strings.Contains(stderr, "replaces") {
		t.Errorf("stderr does not say that running again replaces the password: %q", stderr)
	}
	if strings.Contains(stderr, fallbackNotice) {
		t.Errorf("the plain-password fallback was used on Docker Postgres: %q", stderr)
	}
	u, err := url.Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	if pw, _ := u.User.Password(); u.User.Username() != f.role || len(pw) < 43 || u.RawQuery != "sslmode=disable" {
		t.Errorf("line %s: want user %s, a password of at least 43 characters and the owner's query", first, f.role)
	}
	if err := connectAs(t, first, f.role); err != nil {
		t.Fatalf("the printed line does not log in: %v", err)
	}

	code, second, stderr := f.run(t, defaultPasswordSteps)
	if code != 0 || second == "" || second == first {
		t.Fatalf("second run: exit code %d, same line %v: %s", code, second == first, stderr)
	}
	if err := connectAs(t, second, f.role); err != nil {
		t.Fatalf("the second line does not log in: %v", err)
	}
	var pgErr *pgconn.PgError
	if err := connectAs(t, first, f.role); !errors.As(err, &pgErr) || pgErr.Code != "28P01" {
		t.Errorf("the first password after the second run: %v, want a failed password login (28P01)", err)
	}
}

// When the pre-hashed password does not log in, the plain form is used, said so on stderr, and
// the line is printed only once it logs in. When nothing logs in, or the check itself fails, no
// line is printed and the exit code is not 0.
func TestLoginPasswordVerification(t *testing.T) {
	// wrongHashed and wrongPlain set a password other than the one that will be printed.
	wrongHashed := func(ctx context.Context, db *sql.DB, role, password string) error {
		return setHashedPassword(ctx, db, role, "wrong-"+password)
	}
	wrongPlain := func(ctx context.Context, db *sql.DB, role, password string) error {
		return setPlainPassword(ctx, db, role, "wrong-"+password)
	}
	tests := []struct {
		name       string
		steps      passwordSteps
		wantLine   bool
		wantNotice bool
		wantErr    string
	}{
		{
			name:     "hashed form refused, plain form works",
			steps:    passwordSteps{setHashed: wrongHashed, setPlain: setPlainPassword, login: tryLogin},
			wantLine: true, wantNotice: true,
		},
		{
			name:       "neither form logs in",
			steps:      passwordSteps{setHashed: wrongHashed, setPlain: wrongPlain, login: tryLogin},
			wantNotice: true, wantErr: "does not log in, neither pre-hashed nor plain",
		},
		{
			name: "check fails for another reason",
			steps: passwordSteps{setHashed: setHashedPassword, setPlain: setPlainPassword,
				login: func(context.Context, string) error { return errors.New("network unreachable") }},
			wantErr: "could not check",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newLoginFixture(t)
			code, line, stderr := f.run(t, tt.steps)
			t.Logf("stderr: %s", stderr)
			if tt.wantLine != (line != "") || tt.wantLine != (code == 0) {
				t.Fatalf("exit code %d, line %q; want a line %v (stderr %q)", code, line, tt.wantLine, stderr)
			}
			if got := strings.Contains(stderr, fallbackNotice); got != tt.wantNotice {
				t.Errorf("fallback notice shown %v, want %v: %q", got, tt.wantNotice, stderr)
			}
			if tt.wantErr != "" && !strings.Contains(stderr, tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tt.wantErr)
			}
			if tt.wantLine {
				if err := connectAs(t, line, f.role); err != nil {
					t.Errorf("the printed line does not log in: %v", err)
				}
			}
		})
	}
}

func TestQuoteLiteral(t *testing.T) {
	for in, want := range map[string]string{
		"abc-_09": "'abc-_09'",
		"it's":    "'it''s'",
		`back\sl`: ` E'back\\sl'`,
		"a'b\\c":  ` E'a''b\\c'`,
		"":        "''",
	} {
		if got := quoteLiteral(in); got != want {
			t.Errorf("quoteLiteral(%q) = %q, want %q", in, got, want)
		}
	}
}
