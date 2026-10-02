// Package dbtest connects tests to the local test database (ADR-0026, ADR-0033, ADR-0047).
// Only tests import it.
//
// It reads only TEST_DATABASE_URL and refuses any host that is not 127.0.0.1, localhost or ::1,
// so a test can never reach the development database on Neon. The migrations are applied once
// per test process; test packages run as parallel processes against the same database, and the
// migration lock makes that safe. Tests work inside a transaction that is always rolled back, so
// the shared database stays clean between runs.
package dbtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
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
