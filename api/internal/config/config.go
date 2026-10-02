// Package config reads the API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"
)

// MaxRequestTimeout is the 60 s budget every request must stay inside (ADR-0016, ADR-0020).
const MaxRequestTimeout = 60 * time.Second

const (
	defaultAddr               = "127.0.0.1:8080"
	defaultRequestTimeout     = 50 * time.Second
	defaultLogLevel           = "info"
	defaultDBStatementTimeout = 20 * time.Second
	defaultDBMaxOpenConns     = 10
)

// Secret is a setting that must never be printed or logged: fmt and slog show it as
// [redacted]. Reveal returns the value for the one place that needs it.
type Secret string

// Reveal returns the secret value.
func (s Secret) Reveal() string { return string(s) }

// String hides the value from fmt's %s, %v and %+v.
func (s Secret) String() string { return "[redacted]" }

// GoString hides the value from fmt's %#v.
func (s Secret) GoString() string { return "[redacted]" }

// LogValue hides the value from slog.
func (s Secret) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// Config holds the API settings.
type Config struct {
	// Addr is the listen address (API_ADDR).
	Addr string
	// RequestTimeout is the deadline put on every request's context (REQUEST_TIMEOUT, ADR-0043).
	RequestTimeout time.Duration
	// LogLevel is the minimum slog level (LOG_LEVEL).
	LogLevel slog.Level
	// DatabaseURL is the API's database connection, logging in as app_login (DATABASE_URL).
	// Required.
	DatabaseURL Secret
	// DBStatementTimeout is set on every database transaction (DB_STATEMENT_TIMEOUT, ADR-0043):
	// more than 0 and less than RequestTimeout.
	DBStatementTimeout time.Duration
	// DBMaxOpenConns is the size of the database connection pool (DB_MAX_OPEN_CONNS).
	DBMaxOpenConns int
}

// Load reads the settings from the environment, applies defaults and validates them.
func Load() (Config, error) {
	var cfg Config
	var errs []error

	cfg.Addr = getenv("API_ADDR", defaultAddr)
	if _, _, err := net.SplitHostPort(cfg.Addr); err != nil {
		errs = append(errs, fmt.Errorf("API_ADDR %q: %w", cfg.Addr, err))
	}

	timeout, err := parseRequestTimeout(getenv("REQUEST_TIMEOUT", defaultRequestTimeout.String()))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.RequestTimeout = timeout

	level, err := parseLogLevel(getenv("LOG_LEVEL", defaultLogLevel))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.LogLevel = level

	// The value of DATABASE_URL never goes into an error.
	cfg.DatabaseURL = Secret(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is empty: the API logs in as app_login; "+
			"`make db-login-password` prints the line to put in .env"))
	}

	stmtTimeout, err := parseStatementTimeout(getenv("DB_STATEMENT_TIMEOUT", defaultDBStatementTimeout.String()), timeout)
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DBStatementTimeout = stmtTimeout

	maxConns, err := parsePositiveInt("DB_MAX_OPEN_CONNS", getenv("DB_MAX_OPEN_CONNS", strconv.Itoa(defaultDBMaxOpenConns)))
	if err != nil {
		errs = append(errs, err)
	}
	cfg.DBMaxOpenConns = maxConns

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("config: %w", errors.Join(errs...))
	}
	return cfg, nil
}

func getenv(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func parseRequestTimeout(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("REQUEST_TIMEOUT %q: %w", s, err)
	}
	if d <= 0 || d > MaxRequestTimeout {
		return 0, fmt.Errorf("REQUEST_TIMEOUT %q: must be greater than 0 and at most %s", s, MaxRequestTimeout)
	}
	return d, nil
}

// parseStatementTimeout checks DB_STATEMENT_TIMEOUT. When REQUEST_TIMEOUT is invalid
// (requestTimeout is 0) only the lower bound is checked; that error is reported on its own.
func parseStatementTimeout(s string, requestTimeout time.Duration) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("DB_STATEMENT_TIMEOUT %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("DB_STATEMENT_TIMEOUT %q: must be greater than 0", s)
	}
	if requestTimeout > 0 && d >= requestTimeout {
		return 0, fmt.Errorf("DB_STATEMENT_TIMEOUT %q: must be less than REQUEST_TIMEOUT (%s)", s, requestTimeout)
	}
	return d, nil
}

func parsePositiveInt(name, s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s %q: must be a whole number of at least 1", name, s)
	}
	return n, nil
}

func parseLogLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL %q: must be one of debug, info, warn, error", s)
	}
}
