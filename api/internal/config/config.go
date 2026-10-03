// Package config reads the API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
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
	// The SMTP defaults fit Mailpit in docker-compose.yml (ADR-0026, ADR-0028).
	defaultSMTPHost = "127.0.0.1"
	defaultSMTPPort = 1025
	defaultSMTPFrom = "Satang <noreply@localhost>"
	defaultSMTPTLS  = SMTPTLSNone
)

// SMTPTLS is how the SMTP connection is encrypted (SMTP_TLS). There is no opportunistic mode: a
// server that does not offer STARTTLS is refused under SMTPTLSStartTLS.
type SMTPTLS string

// The SMTP_TLS values.
const (
	// SMTPTLSNone: no encryption (local Mailpit). Not allowed together with credentials.
	SMTPTLSNone SMTPTLS = "none"
	// SMTPTLSStartTLS: plain connection upgraded with STARTTLS, which is mandatory.
	SMTPTLSStartTLS SMTPTLS = "starttls"
	// SMTPTLSImplicit: TLS from the first byte (usually port 465).
	SMTPTLSImplicit SMTPTLS = "tls"
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
	// SMTP is the outgoing mail server (ADR-0028).
	SMTP SMTP
}

// SMTP holds the outgoing mail settings (ADR-0028). Defaults fit local Mailpit.
type SMTP struct {
	// Host and Port are the server (SMTP_HOST, SMTP_PORT).
	Host string
	Port int
	// Username and Password are the login (SMTP_USERNAME, SMTP_PASSWORD); both or neither.
	Username string
	Password Secret
	// From is the sender address (SMTP_FROM), for example "Satang <noreply@example.com>".
	From string
	// TLS is the encryption mode (SMTP_TLS).
	TLS SMTPTLS
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

	smtp, smtpErrs := loadSMTP()
	cfg.SMTP = smtp
	errs = append(errs, smtpErrs...)

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("config: %w", errors.Join(errs...))
	}
	return cfg, nil
}

// loadSMTP reads the SMTP_* settings. SMTP_PASSWORD never goes into an error.
func loadSMTP() (SMTP, []error) {
	var errs []error
	s := SMTP{
		Host:     getenv("SMTP_HOST", defaultSMTPHost),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: Secret(os.Getenv("SMTP_PASSWORD")),
		From:     getenv("SMTP_FROM", defaultSMTPFrom),
		TLS:      SMTPTLS(getenv("SMTP_TLS", string(defaultSMTPTLS))),
	}
	port, err := strconv.Atoi(getenv("SMTP_PORT", strconv.Itoa(defaultSMTPPort)))
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("SMTP_PORT %q: must be a whole number from 1 to 65535", os.Getenv("SMTP_PORT")))
	}
	s.Port = port
	if _, err := mail.ParseAddress(s.From); err != nil {
		errs = append(errs, fmt.Errorf("SMTP_FROM %q: not an email address: %w", s.From, err))
	}
	switch s.TLS {
	case SMTPTLSNone, SMTPTLSStartTLS, SMTPTLSImplicit:
	default:
		errs = append(errs, fmt.Errorf("SMTP_TLS %q: must be one of none, starttls, tls", s.TLS))
	}
	hasUser, hasPassword := s.Username != "", s.Password != ""
	switch {
	case hasUser != hasPassword:
		errs = append(errs, errors.New("SMTP_USERNAME and SMTP_PASSWORD: set both or neither"))
	case hasUser && s.TLS == SMTPTLSNone:
		errs = append(errs, errors.New("SMTP_TLS is none but SMTP_USERNAME and SMTP_PASSWORD are set: "+
			"a password must not go over a clear connection; use starttls or tls"))
	}
	return s, errs
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
