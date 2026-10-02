// Package migrate applies the SQL migrations in db/migrations with goose (ADR-0027).
//
// It is used by the migrate command and by the test database helper. Migrations run as the
// database owner role, never as app_user or app_auth. A Postgres advisory lock held for the whole
// run makes two processes migrating the same database at once safe: the second waits, then finds
// nothing left to apply.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// ErrNothingToRollBack is returned by Down when no migration is applied.
var ErrNothingToRollBack = errors.New("no migration is applied, nothing to roll back")

// lockRetrySeconds and lockRetries bound the wait for another process's migration run:
// one try per second for up to five minutes, and never longer than the caller's context.
const (
	lockRetrySeconds = 1
	lockRetries      = 300
)

// Result is one migration that was applied or rolled back.
type Result struct {
	Version  int64
	Name     string // file name, e.g. 0001_setup_and_roles.sql
	Duration time.Duration
}

// Status is the state of one migration file in the database.
type Status struct {
	Version   int64
	Name      string
	Applied   bool
	AppliedAt time.Time // zero when not applied
}

// Migrator applies the migrations of one directory to one database.
type Migrator struct {
	provider *goose.Provider
}

// New returns a Migrator for the SQL files in dir. The caller owns db and closes it.
func New(db *sql.DB, dir string) (*Migrator, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("migrations directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("migrations directory %s: not a directory", dir)
	}
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockTimeout(lockRetrySeconds, lockRetries))
	if err != nil {
		return nil, fmt.Errorf("migration lock: %w", err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(dir),
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return nil, fmt.Errorf("load migrations from %s: %w", filepath.Clean(dir), err)
	}
	return &Migrator{provider: p}, nil
}

// Up applies every pending migration, in order, and returns those it applied. When one fails,
// it returns the ones applied before it together with the error.
func (m *Migrator) Up(ctx context.Context) ([]Result, error) {
	rs, err := m.provider.Up(ctx)
	if err != nil {
		var partial *goose.PartialError
		if errors.As(err, &partial) {
			rs = partial.Applied
		}
	}
	out := make([]Result, 0, len(rs))
	for _, r := range rs {
		out = append(out, result(r))
	}
	if err != nil {
		return out, fmt.Errorf("migrate up: %w", err)
	}
	return out, nil
}

// Down rolls back the latest applied migration only.
func (m *Migrator) Down(ctx context.Context) (Result, error) {
	r, err := m.provider.Down(ctx)
	if errors.Is(err, goose.ErrNoNextVersion) {
		return Result{}, ErrNothingToRollBack
	}
	if err != nil {
		return Result{}, fmt.Errorf("migrate down: %w", err)
	}
	return result(r), nil
}

// Status lists every migration file and whether it is applied.
func (m *Migrator) Status(ctx context.Context) ([]Status, error) {
	ss, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration status: %w", err)
	}
	out := make([]Status, 0, len(ss))
	for _, s := range ss {
		out = append(out, Status{
			Version:   s.Source.Version,
			Name:      filepath.Base(s.Source.Path),
			Applied:   s.State == goose.StateApplied,
			AppliedAt: s.AppliedAt,
		})
	}
	return out, nil
}

func result(r *goose.MigrationResult) Result {
	return Result{Version: r.Source.Version, Name: filepath.Base(r.Source.Path), Duration: r.Duration}
}
