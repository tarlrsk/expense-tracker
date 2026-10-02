// Package db holds the root database handle and the only ways to use it: WithUserTx and
// WithAuthTx open a transaction as app_user or app_auth, and UserConn and AuthConn hand its
// connection to database adaptors (ADR-0019, ADR-0024, ADR-0032, ADR-0034).
//
// The API logs in as app_login, a role that owns nothing and may only switch to app_user or
// app_auth. Open refuses a connection whose role could read data without switching (Check), so
// a query that somehow ran outside a transaction would see nothing.
//
// The root *gorm.DB is not exported, but UserConn and AuthConn return a GORM handle from which
// GORM would reach the pool (DB(), ConnPool, Connection, Begin ...). Lint rules (forbidigo in
// .golangci.yml) forbid those outside this package. Every transaction sets its role, user and
// timeout with set_config(..., true) at begin, so a connection's leftover session settings
// never decide what a transaction sees.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Pool defaults, used when the matching Config field is zero.
const (
	DefaultMaxOpenConns    = 10
	DefaultConnMaxLifetime = 30 * time.Minute
	DefaultConnMaxIdleTime = 5 * time.Minute
	DefaultSlowThreshold   = 500 * time.Millisecond
)

// Config is what Open needs. URL is a secret: it never appears in errors or logs.
type Config struct {
	// URL is the connection string (DATABASE_URL), logging in as app_login.
	URL string
	// StatementTimeout is set on every transaction (DB_STATEMENT_TIMEOUT, ADR-0043). Required.
	StatementTimeout time.Duration
	// MaxOpenConns limits the pool (DB_MAX_OPEN_CONNS); MaxIdleConns defaults to it.
	MaxOpenConns int
	MaxIdleConns int
	// ConnMaxLifetime and ConnMaxIdleTime recycle connections.
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	// SlowThreshold: statements slower than this are logged at warn level.
	SlowThreshold time.Duration
	// Logger receives GORM's log lines; nil discards them.
	Logger *slog.Logger
}

// DB is the API's database. It implements tx.User and tx.Auth.
type DB struct {
	root    *gorm.DB
	sqlDB   *sql.DB
	timeout string // statement_timeout value, e.g. "20000ms"
}

// Open connects with cfg.URL, checks that the role it logged in as is app_login-like (Check)
// and returns the database. Errors never contain the connection string or its password.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if cfg.URL == "" {
		return nil, errors.New("database: the connection string is empty")
	}
	connCfg, err := pgx.ParseConfig(cfg.URL)
	if err != nil {
		// The parser's message may quote parts of the string, so it is not shown.
		return nil, errors.New("database: the connection string is not a valid Postgres connection string")
	}
	return open(ctx, connCfg, cfg)
}

// open is Open with the connection settings already parsed.
func open(ctx context.Context, connCfg *pgx.ConnConfig, cfg Config) (*DB, error) {
	if cfg.StatementTimeout <= 0 {
		return nil, errors.New("database: the statement timeout must be greater than 0")
	}
	password := connCfg.Password
	sqlDB := stdlib.OpenDB(*connCfg.Copy())
	applyPool(sqlDB, cfg)

	root, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		DisableAutomaticPing:   true,
		Logger:                 newLogger(cfg.Logger, orDefault(cfg.SlowThreshold, DefaultSlowThreshold)),
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, scrub(fmt.Errorf("database: %w", err), password)
	}

	d := &DB{root: root, sqlDB: sqlDB, timeout: timeoutSetting(cfg.StatementTimeout)}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, scrub(fmt.Errorf("database: connect: %w", err), password)
	}
	if err := d.Check(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, scrub(fmt.Errorf("database: %w", err), password)
	}
	return d, nil
}

// Close closes every connection of the pool.
func (d *DB) Close() error {
	if err := d.sqlDB.Close(); err != nil {
		return fmt.Errorf("database: close: %w", err)
	}
	return nil
}

func applyPool(sqlDB *sql.DB, cfg Config) {
	maxOpen := orDefault(cfg.MaxOpenConns, DefaultMaxOpenConns)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(min(orDefault(cfg.MaxIdleConns, maxOpen), maxOpen))
	sqlDB.SetConnMaxLifetime(orDefault(cfg.ConnMaxLifetime, DefaultConnMaxLifetime))
	sqlDB.SetConnMaxIdleTime(orDefault(cfg.ConnMaxIdleTime, DefaultConnMaxIdleTime))
}

// scrub returns err with the password removed from its text, in case a driver ever quotes it.
// errors.Is and errors.As still see the wrapped errors.
func scrub(err error, password string) error {
	if password == "" || !strings.Contains(err.Error(), password) {
		return err
	}
	return scrubbedError{msg: strings.ReplaceAll(err.Error(), password, "[password]"), err: err}
}

type scrubbedError struct {
	msg string
	err error
}

func (e scrubbedError) Error() string { return e.msg }
func (e scrubbedError) Unwrap() error { return e.err }

// orDefault returns v, or fallback when v is zero.
func orDefault[T comparable](v, fallback T) T {
	var zero T
	if v == zero {
		return fallback
	}
	return v
}

// timeoutSetting renders d for statement_timeout in whole milliseconds, at least 1 (0 would
// switch the timeout off).
func timeoutSetting(d time.Duration) string {
	ms := (d + time.Millisecond - 1) / time.Millisecond
	return strconv.FormatInt(int64(max(ms, 1)), 10) + "ms"
}
