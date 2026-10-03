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
var settings = []string{
	"API_ADDR", "REQUEST_TIMEOUT", "LOG_LEVEL", "DATABASE_URL", "DB_STATEMENT_TIMEOUT", "DB_MAX_OPEN_CONNS",
	"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_TLS", "WEB_BASE_URL",
}

const smtpPassword = "smtp-secret-456" //nolint:gosec // fake credentials

func TestLoad(t *testing.T) {
	// defaults is the result with only DATABASE_URL set.
	defaults := Config{
		Addr: "127.0.0.1:8080", RequestTimeout: 50 * time.Second, LogLevel: slog.LevelInfo,
		DatabaseURL: testURL, DBStatementTimeout: 20 * time.Second, DBMaxOpenConns: 10,
		SMTP:       SMTP{Host: "127.0.0.1", Port: 1025, From: "Satang <noreply@localhost>", TLS: SMTPTLSNone},
		WebBaseURL: "http://127.0.0.1:5173",
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
		{
			name: "smtp all set",
			env: env("SMTP_HOST", "smtp.example.com", "SMTP_PORT", "587", "SMTP_USERNAME", "mailer",
				"SMTP_PASSWORD", smtpPassword, "SMTP_FROM", "noreply@example.com", "SMTP_TLS", "starttls"),
			want: with(func(c *Config) {
				c.SMTP = SMTP{
					Host: "smtp.example.com", Port: 587, Username: "mailer", Password: smtpPassword,
					From: "noreply@example.com", TLS: SMTPTLSStartTLS,
				}
			}),
		},
		{
			name: "smtp implicit tls",
			env:  env("SMTP_PORT", "465", "SMTP_TLS", "tls"),
			want: with(func(c *Config) { c.SMTP.Port, c.SMTP.TLS = 465, SMTPTLSImplicit }),
		},
		{name: "smtp port zero", env: env("SMTP_PORT", "0"), wantErr: "SMTP_PORT"},
		{name: "smtp port too big", env: env("SMTP_PORT", "65536"), wantErr: "SMTP_PORT"},
		{name: "smtp port not a number", env: env("SMTP_PORT", "smtp"), wantErr: "SMTP_PORT"},
		{name: "smtp from not an address", env: env("SMTP_FROM", "nobody"), wantErr: "SMTP_FROM"},
		{name: "smtp opportunistic tls", env: env("SMTP_TLS", "opportunistic"), wantErr: "SMTP_TLS"},
		{name: "smtp upper-case tls", env: env("SMTP_TLS", "STARTTLS"), wantErr: "SMTP_TLS"},
		{
			name:    "smtp credentials over a clear connection",
			env:     env("SMTP_USERNAME", "mailer", "SMTP_PASSWORD", smtpPassword),
			wantErr: "must not go over a clear connection",
		},
		{
			name: "smtp username without password", env: env("SMTP_USERNAME", "mailer", "SMTP_TLS", "tls"),
			wantErr: "set both or neither",
		},
		{
			name: "smtp password without username", env: env("SMTP_PASSWORD", smtpPassword, "SMTP_TLS", "tls"),
			wantErr: "set both or neither",
		},
		{
			name: "web base url https with a path and trailing slash", env: env("WEB_BASE_URL", "https://satang.example/app/"),
			want: with(func(c *Config) { c.WebBaseURL = "https://satang.example/app" }),
		},
		{
			name: "web base url http with port", env: env("WEB_BASE_URL", "http://192.168.1.20:5173"),
			want: with(func(c *Config) { c.WebBaseURL = "http://192.168.1.20:5173" }),
		},
		{name: "web base url without scheme", env: env("WEB_BASE_URL", "satang.example"), wantErr: "WEB_BASE_URL"},
		{name: "web base url host and port only", env: env("WEB_BASE_URL", "127.0.0.1:5173"), wantErr: "WEB_BASE_URL"},
		{name: "web base url relative", env: env("WEB_BASE_URL", "/app"), wantErr: "WEB_BASE_URL"},
		{name: "web base url other scheme", env: env("WEB_BASE_URL", "ftp://satang.example"), wantErr: "WEB_BASE_URL"},
		{name: "web base url without host", env: env("WEB_BASE_URL", "https:///app"), wantErr: "WEB_BASE_URL"},
		{name: "web base url with query", env: env("WEB_BASE_URL", "https://satang.example/?a=1"), wantErr: "query or a fragment"},
		{name: "web base url with empty query", env: env("WEB_BASE_URL", "https://satang.example/?"), wantErr: "query or a fragment"},
		{name: "web base url with fragment", env: env("WEB_BASE_URL", "https://satang.example/#x"), wantErr: "query or a fragment"},
		{name: "web base url with empty fragment", env: env("WEB_BASE_URL", "https://satang.example/#"), wantErr: "query or a fragment"},
		{
			name: "web base url with credentials", env: env("WEB_BASE_URL", "https://user:pw-secret-123@satang.example"),
			wantErr: "user name or password",
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
				if strings.Contains(err.Error(), "pw-secret-123") || strings.Contains(err.Error(), smtpPassword) {
					t.Errorf("error leaks a secret: %v", err)
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

// The database URL and the SMTP password never show in printed or logged config.
func TestSecretIsHidden(t *testing.T) {
	cfg := Config{DatabaseURL: testURL, SMTP: SMTP{Password: smtpPassword}}
	var log strings.Builder
	slog.New(slog.NewTextHandler(&log, nil)).Info("config", slog.Any("cfg", cfg), slog.Any("url", cfg.DatabaseURL))
	for _, out := range []string{
		fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg), fmt.Sprintf("url=%s", cfg.DatabaseURL),
		fmt.Sprintf("%+v", cfg.SMTP), log.String(),
	} {
		if strings.Contains(out, "pw-secret-123") || strings.Contains(out, smtpPassword) {
			t.Errorf("secret printed: %s", out)
		}
	}
	if cfg.DatabaseURL.Reveal() != testURL {
		t.Error("Reveal does not return the value")
	}
}
