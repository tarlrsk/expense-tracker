package config

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

const testURL = "postgres://app_login:pw-secret-123@db.example.com/app?sslmode=require" //nolint:gosec // fake credentials

// settings are every variable Load reads.
var settings = []string{"API_ADDR", "REQUEST_TIMEOUT", "LOG_LEVEL", "DATABASE_URL", "DB_STATEMENT_TIMEOUT", "DB_MAX_OPEN_CONNS"}

func TestLoad(t *testing.T) {
	// defaults is the result with only DATABASE_URL set.
	defaults := Config{
		Addr: "127.0.0.1:8080", RequestTimeout: 50 * time.Second, LogLevel: slog.LevelInfo,
		DatabaseURL: testURL, DBStatementTimeout: 20 * time.Second, DBMaxOpenConns: 10,
	}
	with := func(change func(*Config)) Config {
		c := defaults
		change(&c)
		return c
	}
	db := map[string]string{"DATABASE_URL": testURL}
	// env is DATABASE_URL plus name, value pairs.
	env := func(pairs ...string) map[string]string {
		m := map[string]string{"DATABASE_URL": testURL}
		for len(pairs) >= 2 {
			m[pairs[0]] = pairs[1]
			pairs = pairs[2:]
		}
		return m
	}

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr string
	}{
		{name: "defaults", env: db, want: defaults},
		{
			name: "all set",
			env: env("API_ADDR", "0.0.0.0:9000", "REQUEST_TIMEOUT", "30s", "LOG_LEVEL", "debug",
				"DB_STATEMENT_TIMEOUT", "5s", "DB_MAX_OPEN_CONNS", "4"),
			want: with(func(c *Config) {
				c.Addr, c.RequestTimeout, c.LogLevel = "0.0.0.0:9000", 30*time.Second, slog.LevelDebug
				c.DBStatementTimeout, c.DBMaxOpenConns = 5*time.Second, 4
			}),
		},
		{
			name: "timeout at the 60s limit",
			env:  env("REQUEST_TIMEOUT", "60s"),
			want: with(func(c *Config) { c.RequestTimeout = 60 * time.Second }),
		},
		{name: "warn level", env: env("LOG_LEVEL", "warn"), want: with(func(c *Config) { c.LogLevel = slog.LevelWarn })},
		{name: "error level", env: env("LOG_LEVEL", "error"), want: with(func(c *Config) { c.LogLevel = slog.LevelError })},
		{
			name: "statement timeout just under the request timeout",
			env:  env("REQUEST_TIMEOUT", "10s", "DB_STATEMENT_TIMEOUT", "9999ms"),
			want: with(func(c *Config) { c.RequestTimeout, c.DBStatementTimeout = 10*time.Second, 9999*time.Millisecond }),
		},
		{name: "timeout over 60s", env: env("REQUEST_TIMEOUT", "61s"), wantErr: "REQUEST_TIMEOUT"},
		{name: "timeout zero", env: env("REQUEST_TIMEOUT", "0s"), wantErr: "REQUEST_TIMEOUT"},
		{name: "timeout negative", env: env("REQUEST_TIMEOUT", "-5s"), wantErr: "REQUEST_TIMEOUT"},
		{name: "timeout not a duration", env: env("REQUEST_TIMEOUT", "50"), wantErr: "REQUEST_TIMEOUT"},
		{name: "unknown log level", env: env("LOG_LEVEL", "verbose"), wantErr: "LOG_LEVEL"},
		{name: "upper-case log level", env: env("LOG_LEVEL", "INFO"), wantErr: "LOG_LEVEL"},
		{name: "addr without port", env: env("API_ADDR", "localhost"), wantErr: "API_ADDR"},
		{name: "no DATABASE_URL", env: map[string]string{}, wantErr: "DATABASE_URL is empty"},
		{name: "statement timeout zero", env: env("DB_STATEMENT_TIMEOUT", "0s"), wantErr: "DB_STATEMENT_TIMEOUT"},
		{name: "statement timeout negative", env: env("DB_STATEMENT_TIMEOUT", "-1s"), wantErr: "DB_STATEMENT_TIMEOUT"},
		{name: "statement timeout not a duration", env: env("DB_STATEMENT_TIMEOUT", "20"), wantErr: "DB_STATEMENT_TIMEOUT"},
		{name: "statement timeout equal to request timeout", env: env("DB_STATEMENT_TIMEOUT", "50s"), wantErr: "less than REQUEST_TIMEOUT"},
		{
			name: "statement timeout over a shorter request timeout", env: env("REQUEST_TIMEOUT", "10s", "DB_STATEMENT_TIMEOUT", "20s"),
			wantErr: "less than REQUEST_TIMEOUT",
		},
		{name: "pool size zero", env: env("DB_MAX_OPEN_CONNS", "0"), wantErr: "DB_MAX_OPEN_CONNS"},
		{name: "pool size not a number", env: env("DB_MAX_OPEN_CONNS", "ten"), wantErr: "DB_MAX_OPEN_CONNS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range settings {
				t.Setenv(name, tt.env[name])
			}
			got, err := Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() = %+v, want an error", got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %v, want it to mention %q", err, tt.wantErr)
				}
				if strings.Contains(err.Error(), "pw-secret-123") {
					t.Errorf("error leaks DATABASE_URL: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// The database URL never shows in printed or logged config.
func TestSecretIsHidden(t *testing.T) {
	cfg := Config{DatabaseURL: testURL}
	var log strings.Builder
	slog.New(slog.NewTextHandler(&log, nil)).Info("config", slog.Any("cfg", cfg), slog.Any("url", cfg.DatabaseURL))
	for _, out := range []string{
		fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), fmt.Sprintf("url=%s", cfg.DatabaseURL),
		log.String(),
	} {
		if strings.Contains(out, "pw-secret-123") {
			t.Errorf("secret printed: %s", out)
		}
	}
	if cfg.DatabaseURL.Reveal() != testURL {
		t.Error("Reveal does not return the value")
	}
}
