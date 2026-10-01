package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults",
			env:  map[string]string{},
			want: Config{Addr: "127.0.0.1:8080", RequestTimeout: 50 * time.Second, LogLevel: slog.LevelInfo},
		},
		{
			name: "all set",
			env:  map[string]string{"API_ADDR": "0.0.0.0:9000", "REQUEST_TIMEOUT": "30s", "LOG_LEVEL": "debug"},
			want: Config{Addr: "0.0.0.0:9000", RequestTimeout: 30 * time.Second, LogLevel: slog.LevelDebug},
		},
		{
			name: "timeout at the 60s limit",
			env:  map[string]string{"REQUEST_TIMEOUT": "60s"},
			want: Config{Addr: "127.0.0.1:8080", RequestTimeout: 60 * time.Second, LogLevel: slog.LevelInfo},
		},
		{
			name: "warn level",
			env:  map[string]string{"LOG_LEVEL": "warn"},
			want: Config{Addr: "127.0.0.1:8080", RequestTimeout: 50 * time.Second, LogLevel: slog.LevelWarn},
		},
		{
			name: "error level",
			env:  map[string]string{"LOG_LEVEL": "error"},
			want: Config{Addr: "127.0.0.1:8080", RequestTimeout: 50 * time.Second, LogLevel: slog.LevelError},
		},
		{name: "timeout over 60s", env: map[string]string{"REQUEST_TIMEOUT": "61s"}, wantErr: true},
		{name: "timeout zero", env: map[string]string{"REQUEST_TIMEOUT": "0s"}, wantErr: true},
		{name: "timeout negative", env: map[string]string{"REQUEST_TIMEOUT": "-5s"}, wantErr: true},
		{name: "timeout not a duration", env: map[string]string{"REQUEST_TIMEOUT": "50"}, wantErr: true},
		{name: "unknown log level", env: map[string]string{"LOG_LEVEL": "verbose"}, wantErr: true},
		{name: "upper-case log level", env: map[string]string{"LOG_LEVEL": "INFO"}, wantErr: true},
		{name: "addr without port", env: map[string]string{"API_ADDR": "localhost"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range []string{"API_ADDR", "REQUEST_TIMEOUT", "LOG_LEVEL"} {
				t.Setenv(name, tt.env[name])
			}
			got, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() = %+v, want an error", got)
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
