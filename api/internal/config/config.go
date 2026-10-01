// Package config reads the API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"
)

// MaxRequestTimeout is the 60 s budget every request must stay inside (ADR-0016, ADR-0020).
const MaxRequestTimeout = 60 * time.Second

const (
	defaultAddr           = "127.0.0.1:8080"
	defaultRequestTimeout = 50 * time.Second
	defaultLogLevel       = "info"
)

// Config holds the API settings.
type Config struct {
	// Addr is the listen address (API_ADDR).
	Addr string
	// RequestTimeout is the deadline put on every request's context (REQUEST_TIMEOUT, ADR-0043).
	RequestTimeout time.Duration
	// LogLevel is the minimum slog level (LOG_LEVEL).
	LogLevel slog.Level
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
