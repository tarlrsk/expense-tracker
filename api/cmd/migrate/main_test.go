package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	const secret = "s3cret-pw"
	tests := []struct {
		name     string
		args     []string
		url      string
		wantCode int
		wantErr  string
	}{
		{name: "no command", args: nil, wantCode: 2, wantErr: "usage"},
		{name: "unknown command", args: []string{"sideways"}, wantCode: 2, wantErr: "unknown command"},
		{name: "two commands", args: []string{"up", "down"}, wantCode: 2, wantErr: "usage"},
		{name: "empty DATABASE_URL", args: []string{"status"}, url: "", wantCode: 1, wantErr: "DATABASE_URL is empty"},
		{name: "invalid DATABASE_URL", args: []string{"up"}, url: "postgres://u:" + secret + "@[bad", wantCode: 1, wantErr: "not a valid"},
		{
			// Port 1 refuses at once; the error must not carry the password.
			name: "unreachable database", args: []string{"status"},
			url:      "postgres://u:" + secret + "@127.0.0.1:1/x?sslmode=disable&connect_timeout=5",
			wantCode: 1, wantErr: "connect",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(name string) string {
				if name == "DATABASE_URL" {
					return tt.url
				}
				t.Errorf("read setting %s; only DATABASE_URL is allowed", name)
				return ""
			}
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), tt.args, getenv, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantErr)
			}
			if out := stdout.String() + stderr.String(); strings.Contains(out, secret) {
				t.Errorf("output leaks the password: %q", out)
			}
		})
	}
}

func TestScrub(t *testing.T) {
	const password = "p@ss"
	dsn := "postgres://u:" + "p%40ss" + "@h/db"
	got := scrub("failed for "+dsn+" with "+password, dsn, password)
	if strings.Contains(got, password) || strings.Contains(got, dsn) {
		t.Fatalf("scrub left a secret: %q", got)
	}
}
