// Command migrate applies the SQL migrations in db/migrations to the database in
// MIGRATION_DATABASE_URL, the owner's connection (ADR-0027). It reads no other setting and never
// prints the connection string or its password.
//
//	migrate [-dir ../db/migrations] up|down|status|login-password
//
// up applies every pending migration, down rolls back the latest one only, status lists them.
// login-password gives the API's login role (app_login) a new random password, checks that it
// logs in, and only then prints the DATABASE_URL line for .env; running it again replaces the
// password.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

const (
	defaultDir = "../db/migrations"
	// envVar is the only setting this command reads: the owner's connection.
	envVar = "MIGRATION_DATABASE_URL"
	// loginRole is the role the API logs in as (migration 0001).
	loginRole = "app_login"
	// passwordBytes random bytes make the login password (43 URL-safe characters).
	passwordBytes = 32
	// scramIterations matches Postgres' default scram_iterations.
	scramIterations = 4096
	// loginCheckTimeout bounds each test login with the new password.
	loginCheckTimeout = 15 * time.Second
	// loginTries test logins, loginPause apart, are made with each password form before it is
	// judged not to work: a proxy in front of the server may need a moment to see a new password.
	loginTries = 5
	loginPause = time.Second
	// changeTimeout covers up and down, including a cold Neon start and waiting for the
	// migration lock; statusTimeout covers status. login-password gets changeTimeout too: up to
	// twice loginTries logins with pauses.
	changeTimeout = 2 * time.Minute
	statusTimeout = 30 * time.Second
)

const usage = "usage: migrate [-dir DIR] up|down|status|login-password"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is main without the process: it returns the exit code.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	return runWith(ctx, args, getenv, stdout, stderr, loginRole, defaultPasswordSteps)
}

// runWith is run with the login role and the login-password steps as parameters, so a test can
// use a throw-away role and make a step fail.
func runWith(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer,
	role string, steps passwordSteps,
) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", defaultDir, "directory with the SQL migration files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	command := fs.Arg(0)
	timeout := changeTimeout
	switch command {
	case "up", "down", "login-password":
	case "status":
		timeout = statusTimeout
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n%s\n", command, usage)
		return 2
	}

	dsn := getenv(envVar)
	if dsn == "" {
		_, _ = fmt.Fprintln(stderr, "migrate: "+envVar+" is empty; set it in .env (the database owner's connection) "+
			"and use the make targets, or export it")
		return 1
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		// The parser's message may quote parts of the string, so it is not shown.
		_, _ = fmt.Fprintln(stderr, "migrate: "+envVar+" is not a valid Postgres connection string")
		return 1
	}
	if host, ok := poolerHost(cfg); ok {
		_, _ = fmt.Fprintf(stderr, "migrate: host %s is a connection pooler; use the direct host (without -pooler) "+
			"for migrations (ADR-0026): the migration lock does not work through PgBouncer\n", host)
		return 1
	}
	var loginURL *url.URL
	if command == "login-password" {
		if loginURL, err = loginURLBase(dsn, cfg.Database); err != nil {
			_, _ = fmt.Fprintln(stderr, "migrate:", err)
			return 1
		}
	}
	// Anything printed from here on is scrubbed of the password and the string itself.
	fail := func(err error) int {
		_, _ = fmt.Fprintln(stderr, "migrate:", scrub(err.Error(), dsn, cfg.Password))
		return 1
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	db := stdlib.OpenDB(*cfg)
	defer func() { _ = db.Close() }()
	if command == "login-password" {
		// stdout carries only the line for .env.
		_, _ = fmt.Fprintf(stderr, "database %s on %s\n", cfg.Database, cfg.Host)
	} else {
		_, _ = fmt.Fprintf(stdout, "database %s on %s\n", cfg.Database, cfg.Host)
	}
	if err := db.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("connect: %w", err))
	}

	if command == "login-password" {
		line, err := setLoginPassword(ctx, db, loginURL, role, steps, stderr)
		if err != nil {
			return fail(err)
		}
		_, _ = fmt.Fprintf(stderr, "Set a new password for role %s. Running this again replaces it, and the old "+
			"DATABASE_URL line stops working.\nPut this line in .env, replacing any DATABASE_URL line there:\n", role)
		_, _ = fmt.Fprintln(stdout, line)
		return 0
	}

	m, err := migrate.New(db, *dir)
	if err != nil {
		return fail(err)
	}

	switch command {
	case "up":
		rs, err := m.Up(ctx)
		for _, r := range rs {
			_, _ = fmt.Fprintf(stdout, "applied     %s (%s)\n", r.Name, r.Duration.Round(time.Millisecond))
		}
		if err != nil {
			return fail(err)
		}
		if len(rs) == 0 {
			_, _ = fmt.Fprintln(stdout, "up to date, nothing to apply")
		}
	case "down":
		r, err := m.Down(ctx)
		if errors.Is(err, migrate.ErrNothingToRollBack) {
			_, _ = fmt.Fprintln(stdout, "nothing to roll back")
			return 0
		}
		if err != nil {
			return fail(err)
		}
		_, _ = fmt.Fprintf(stdout, "rolled back %s (%s)\n", r.Name, r.Duration.Round(time.Millisecond))
	case "status":
		ss, err := m.Status(ctx)
		if err != nil {
			return fail(err)
		}
		for _, s := range ss {
			state := "pending"
			if s.Applied {
				state = "applied " + s.AppliedAt.UTC().Format(time.RFC3339)
			}
			_, _ = fmt.Fprintf(stdout, "%-30s %s\n", s.Name, state)
		}
	}
	return 0
}

// loginURLBase is the migration URL without its user and password: host, port, database and
// query parameters stay the same for the API's connection, except the owner's own login
// details. database fills in a missing name. Errors never quote the URL.
func loginURLBase(dsn, database string) (*url.URL, error) {
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || u.Opaque != "" {
		return nil, errors.New("login-password needs " + envVar + " in URL form (postgres://user:password@host/database?...)")
	}
	if strings.Trim(u.Path, "/") == "" {
		u.Path = "/" + database
	}
	u.User = nil
	u.Fragment, u.RawFragment = "", ""
	// The owner's name, password or client certificate may also be given as query parameters;
	// they log in the owner, not app_login, and must not be copied into the API's line. The
	// server's certificate settings (sslmode, sslrootcert) stay.
	q := u.Query()
	changed := false
	for _, key := range ownerOnlyParams {
		if q.Has(key) {
			q.Del(key)
			changed = true
		}
	}
	if changed {
		u.RawQuery = q.Encode()
	}
	return u, nil
}

// ownerOnlyParams are the connection parameters that belong to the owner's login.
var ownerOnlyParams = []string{"user", "password", "passfile", "sslcert", "sslkey", "sslpassword"}

// passwordSteps are the steps of login-password that touch the server; tests replace them.
type passwordSteps struct {
	// setHashed sets password on role by sending only its SCRAM-SHA-256 verifier.
	setHashed func(ctx context.Context, db *sql.DB, role, password string) error
	// setPlain sets password on role as plain text (the server hashes it).
	setPlain func(ctx context.Context, db *sql.DB, role, password string) error
	// login opens a fresh connection with connURL and runs select 1.
	login func(ctx context.Context, connURL string) error
	// tries and pause: how often login is tried for one password form, and the wait between.
	tries int
	pause time.Duration
}

var defaultPasswordSteps = passwordSteps{
	setHashed: setHashedPassword, setPlain: setPlainPassword, login: tryLogin,
	tries: loginTries, pause: loginPause,
}

// setLoginPassword gives role a new random password and returns the DATABASE_URL line for .env,
// built on base, only after a fresh connection with exactly that line has logged in.
//
// It first sends only a SCRAM-SHA-256 verifier, so the password itself never reaches the server
// or its logs. The server may not take that form (a hosted Postgres may need the plain
// password): when the ALTER ROLE fails for any reason but "no such role" (42704) or "not
// allowed" (42501), or the line still does not log in after steps.tries tries with an
// authentication error (SQLSTATE class 28), the same password is set again in plain form,
// notice says so, and the login is tried again the same way. Any other failure returns an
// error and no line. A line that could not be pasted into .env as written (it would contain a
// single quote) is refused before anything changes on the server. Errors never contain the
// password.
func setLoginPassword(ctx context.Context, db *sql.DB, base *url.URL, role string, steps passwordSteps,
	notice io.Writer,
) (string, error) {
	raw := make([]byte, passwordBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("random password: %w", err)
	}
	password := base64.RawURLEncoding.EncodeToString(raw)
	u := *base
	u.User = url.UserPassword(role, password)
	connURL := u.String()
	if strings.Contains(connURL, "'") {
		return "", fmt.Errorf("the DATABASE_URL line for %s would contain a single quote (from the host, database "+
			"or query parameters of %s), which .env cannot hold inside '...'; nothing was changed on the server; "+
			"remove or percent-encode it (%%27), then run this again", role, envVar)
	}
	hide := func(err error) string {
		return strings.ReplaceAll(strings.ReplaceAll(err.Error(), connURL, "[DATABASE_URL]"), password, "[password]")
	}

	err := steps.setHashed(ctx, db, role, password)
	if err == nil {
		err = loginRetrying(ctx, connURL, steps)
		if err == nil {
			return "DATABASE_URL='" + connURL + "'", nil
		}
		if !isAuthError(err) {
			return "", couldNotCheck(role, hide(err))
		}
	} else if isFatalAlterError(err) {
		return "", alterRoleError(role, err)
	}

	// The pre-hashed form was refused, or does not log in: set the plain form.
	if err := steps.setPlain(ctx, db, role, password); err != nil {
		return "", alterRoleError(role, err)
	}
	_, _ = fmt.Fprintln(notice, "The server did not accept a pre-hashed password, so the plain password was set "+
		"instead, sent over the database connection (encrypted when the URL requires TLS).")
	err = loginRetrying(ctx, connURL, steps)
	switch {
	case isAuthError(err):
		return "", fmt.Errorf("the new password of %s does not log in, neither pre-hashed nor plain (%s); "+
			"no DATABASE_URL line was printed and the old one no longer works; check the server's "+
			"authentication settings, then run this again", role, hide(err))
	case err != nil:
		return "", couldNotCheck(role, hide(err))
	}
	return "DATABASE_URL='" + connURL + "'", nil
}

func couldNotCheck(role, reason string) error {
	return fmt.Errorf("could not check that %s logs in with the new password (%s); "+
		"no DATABASE_URL line was printed and the old one may no longer work; run this again", role, reason)
}

// loginRetrying tries steps.login up to steps.tries times, steps.pause apart, and returns nil at
// the first success, otherwise the last error. It stops early when ctx ends.
func loginRetrying(ctx context.Context, connURL string, steps passwordSteps) error {
	var err error
	for i := range max(steps.tries, 1) {
		if i > 0 {
			select {
			case <-ctx.Done():
				return errors.Join(err, ctx.Err())
			case <-time.After(steps.pause):
			}
		}
		if err = steps.login(ctx, connURL); err == nil {
			return nil
		}
	}
	return err
}

// isFatalAlterError reports whether a failed ALTER ROLE means the plain form would fail too:
// the role does not exist (42704) or the owner may not change it (42501).
func isFatalAlterError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "42704" || pgErr.Code == "42501")
}

// setHashedPassword sends only the SCRAM-SHA-256 verifier of password.
func setHashedPassword(ctx context.Context, db *sql.DB, role, password string) error {
	verifier, err := scramVerifier(password)
	if err != nil {
		return err
	}
	return alterRolePassword(ctx, db, role, verifier)
}

// setPlainPassword sends password itself; the server hashes it.
func setPlainPassword(ctx context.Context, db *sql.DB, role, password string) error {
	return alterRolePassword(ctx, db, role, password)
}

// alterRolePassword runs ALTER ROLE ... PASSWORD. The statement takes no parameters: the name is
// quoted by pgx.Identifier and the value by quoteLiteral.
func alterRolePassword(ctx context.Context, db *sql.DB, role, value string) error {
	q := "alter role " + pgx.Identifier{role}.Sanitize() + " password " + quoteLiteral(value) //nolint:gosec // quoted, see above
	if _, err := db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("alter role: %w", err)
	}
	return nil
}

// quoteLiteral quotes s as an SQL string literal: single quotes doubled, and the E-prefixed form with
// doubled backslashes when s has a backslash, so it is right whatever
// standard_conforming_strings says.
func quoteLiteral(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	if strings.Contains(s, `\`) {
		return ` E'` + strings.ReplaceAll(s, `\`, `\\`) + `'`
	}
	return "'" + s + "'"
}

// alterRoleError explains a failed ALTER ROLE. Its text never contains the password: Postgres
// does not echo the statement in its error messages.
func alterRoleError(role string, err error) error {
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "42704":
		return fmt.Errorf("role %s does not exist; run make migrate first", role)
	case errors.As(err, &pgErr) && pgErr.Code == "42501":
		return fmt.Errorf("the role in %s may not change the password of %s: %w", envVar, role, err)
	}
	return fmt.Errorf("set the password of %s: %w", role, err)
}

// tryLogin opens a fresh connection with connURL (the line about to be printed: same host, port,
// database and TLS settings) and runs select 1.
func tryLogin(ctx context.Context, connURL string) error {
	cfg, err := pgx.ParseConfig(connURL)
	if err != nil {
		return errors.New("the new DATABASE_URL is not a valid connection string")
	}
	ctx, cancel := context.WithTimeout(ctx, loginCheckTimeout)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("log in: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	var one int
	if err := conn.QueryRow(ctx, "select 1").Scan(&one); err != nil {
		return fmt.Errorf("select 1: %w", err)
	}
	return nil
}

// isAuthError reports whether err is a Postgres authentication failure (SQLSTATE class 28).
func isAuthError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, "28")
}

// scramVerifier returns the SCRAM-SHA-256 verifier Postgres stores for password (RFC 5802,
// RFC 7677), in the form ALTER ROLE ... PASSWORD accepts as already hashed.
func scramVerifier(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("random salt: %w", err)
	}
	salted, err := pbkdf2.Key(sha256.New, password, salt, scramIterations, sha256.Size)
	if err != nil {
		return "", fmt.Errorf("hash the password: %w", err)
	}
	clientKey := hmacSHA256(salted, "Client Key")
	storedKey := sha256.Sum256(clientKey)
	serverKey := hmacSHA256(salted, "Server Key")
	b64 := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s", scramIterations, b64(salt), b64(storedKey[:]), b64(serverKey)), nil
}

func hmacSHA256(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(msg))
	return h.Sum(nil)
}

// poolerHost returns the first host that is a Neon connection pooler (its name contains -pooler).
func poolerHost(cfg *pgx.ConnConfig) (string, bool) {
	hosts := []string{cfg.Host}
	for _, fb := range cfg.Fallbacks {
		hosts = append(hosts, fb.Host)
	}
	for _, h := range hosts {
		if strings.Contains(strings.ToLower(h), "-pooler") {
			return h, true
		}
	}
	return "", false
}

// scrub removes the connection string and the password from msg.
func scrub(msg, dsn, password string) string {
	msg = strings.ReplaceAll(msg, dsn, "["+envVar+"]")
	if password != "" {
		msg = strings.ReplaceAll(msg, password, "[password]")
	}
	return msg
}
