// Package dbtest connects tests to the local test database (ADR-0026, ADR-0033, ADR-0047).
// Only tests import it.
//
// It reads only TEST_DATABASE_URL and refuses any host that is not 127.0.0.1, localhost or ::1,
// so a test can never reach the development database on Neon. The migrations are applied once
// per test process; test packages run as parallel processes against the same database, and the
// migration lock makes that safe. Tests work inside a transaction that is always rolled back, so
// the shared database stays clean between runs.
//
// DB connects as the test superuser (for the schema tests); LoginConfig connects as app_login,
// the role the API uses, with a fixed password that is set on the test server only.
package dbtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

// EnvVar is the only setting this package reads.
const EnvVar = "TEST_DATABASE_URL"

// LocalHosts are the only hosts a test may connect to.
var LocalHosts = []string{"127.0.0.1", "localhost", "::1"}

// migrateTimeout bounds opening the database and applying the migrations once per process.
const migrateTimeout = 2 * time.Minute

// ParseLocal parses a test connection string and refuses it unless it points at exactly one
// local host and at a database whose name contains "test" (so a local port-forward to a real
// database does not pass). The checks run on the configuration that would really be used (pgx also reads
// PGHOST and similar variables), plus the raw string for several hosts or a host= parameter.
// Error messages never quote the connection string.
func ParseLocal(connString string) (*pgx.ConnConfig, error) {
	if strings.TrimSpace(connString) == "" {
		return nil, fmt.Errorf("%s is empty", EnvVar)
	}
	if err := checkRaw(connString); err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("%s is not a valid Postgres connection string", EnvVar)
	}
	hosts := []string{cfg.Host}
	for _, fb := range cfg.Fallbacks {
		if !slices.Contains(hosts, fb.Host) {
			hosts = append(hosts, fb.Host)
		}
	}
	if len(hosts) > 1 {
		return nil, fmt.Errorf("%s names several hosts; use one local host", EnvVar)
	}
	if !slices.Contains(LocalHosts, hosts[0]) {
		return nil, fmt.Errorf("%s host %q is not local; allowed: %s", EnvVar, hosts[0], strings.Join(LocalHosts, ", "))
	}
	if !strings.Contains(strings.ToLower(cfg.Database), "test") {
		return nil, fmt.Errorf("%s database %q is not a test database; its name must contain \"test\"", EnvVar, cfg.Database)
	}
	return cfg, nil
}

// checkRaw refuses a URL with several hosts or a host/hostaddr parameter that is not local.
func checkRaw(connString string) error {
	if !strings.HasPrefix(connString, "postgres://") && !strings.HasPrefix(connString, "postgresql://") {
		return nil // keyword=value form: the parsed configuration is checked instead
	}
	u, err := url.Parse(connString)
	if err != nil {
		return fmt.Errorf("%s is not a valid Postgres connection string", EnvVar)
	}
	if strings.Contains(u.Host, ",") {
		return fmt.Errorf("%s names several hosts; use one local host", EnvVar)
	}
	for _, key := range []string{"host", "hostaddr"} {
		for _, v := range u.Query()[key] {
			if !slices.Contains(LocalHosts, v) {
				return fmt.Errorf("%s parameter %s=%q is not local; allowed: %s", EnvVar, key, v, strings.Join(LocalHosts, ", "))
			}
		}
	}
	return nil
}

// shared is the database of this test process, opened and migrated once.
var shared struct {
	once sync.Once
	db   *sql.DB
	err  error
}

// DB returns the migrated test database, shared by the whole test process.
// Without TEST_DATABASE_URL it fails the test (skips it under -short); it fails it when the URL
// is not a local test database.
func DB(t testing.TB) *sql.DB {
	t.Helper()
	cfg := Config(t)
	shared.once.Do(func() {
		shared.db, shared.err = openAndMigrate(cfg)
	})
	if shared.err != nil {
		t.Fatalf("test database: %v", shared.err)
	}
	return shared.db
}

// Config returns a copy of the checked test connection settings, for tests that need their own
// connection (for example to a throw-away database). It skips and fails like DB.
func Config(t testing.TB) *pgx.ConnConfig {
	t.Helper()
	raw := os.Getenv(EnvVar)
	if raw == "" {
		if testing.Short() {
			t.Skipf("%s is not set and -short is on: skipping a database test", EnvVar)
		}
		t.Fatalf("%s is not set: run `make test` (it starts the test database), "+
			"or `go test -short ./...` to skip the database tests", EnvVar)
	}
	cfg, err := ParseLocal(raw)
	if err != nil {
		t.Fatalf("refusing to connect: %v", err)
	}
	return cfg
}

// LoginRole is the role the API logs in as (made by migration 0001 without a password).
const LoginRole = "app_login"

// loginTestPassword is the fixed password tests give app_login on the test server only. The real
// password is set by `make db-login-password` on the development database.
const loginTestPassword = "app-login-test-only" //nolint:gosec // local Docker test server only

// loginLockKey serialises setting that password across test processes (roles are server-wide).
const loginLockKey = 0x61707031 // "app1"

var login struct {
	once sync.Once
	err  error
}

// LoginConfig returns the test connection settings logged in as app_login, the role the API
// uses, on the migrated test database. The first call per process makes sure the role can log
// in: it tries to, and only when that fails sets the test password, under an advisory lock so
// parallel test processes do not update the role at the same time. It skips and fails like DB.
func LoginConfig(t testing.TB) *pgx.ConnConfig {
	t.Helper()
	shared := DB(t)
	cfg := Config(t)
	cfg.User, cfg.Password = LoginRole, loginTestPassword
	login.once.Do(func() { login.err = ensureLogin(shared, cfg) })
	if login.err != nil {
		t.Fatalf("log in as %s: %v", LoginRole, login.err)
	}
	return cfg
}

// LoginURL is LoginConfig as a connection string, for tests that open the database the way the
// API does (db.Open with DATABASE_URL). It skips and fails like DB.
func LoginURL(t testing.TB) string {
	t.Helper()
	cfg := LoginConfig(t)
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:     "/" + cfg.Database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func ensureLogin(shared *sql.DB, cfg *pgx.ConnConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()
	if canLogIn(ctx, cfg) == nil {
		return nil
	}
	tx, err := shared.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "select pg_advisory_xact_lock($1)", loginLockKey); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	// Another process may have set it while this one waited for the lock.
	if canLogIn(ctx, cfg) != nil {
		// ALTER ROLE takes no parameters; both parts are constants.
		q := "alter role " + pgx.Identifier{LoginRole}.Sanitize() + " password '" + loginTestPassword + "'" //nolint:gosec // see above
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("set the test password: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return canLogIn(ctx, cfg)
}

func canLogIn(ctx context.Context, cfg *pgx.ConnConfig) error {
	conn, err := pgx.ConnectConfig(ctx, cfg.Copy())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return conn.Close(ctx)
}

// Tx begins a transaction on db that is always rolled back when the test ends.
func Tx(t testing.TB, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(context.WithoutCancel(t.Context()), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("rollback: %v", err)
		}
	})
	return tx
}

// MigrationsDir finds db/migrations by walking up from the working directory.
func MigrationsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	for {
		candidate := filepath.Join(dir, "db", "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("db/migrations not found above the working directory")
		}
		dir = parent
	}
}

func openAndMigrate(cfg *pgx.ConnConfig) (*sql.DB, error) {
	dir, err := MigrationsDir()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()

	db := stdlib.OpenDB(*cfg.Copy())
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect (is postgres-test running? `docker compose up -d --wait postgres-test`): %w", err)
	}
	m, err := migrate.New(db, dir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := m.Up(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
