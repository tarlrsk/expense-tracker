package dbtest

import (
	"strings"
	"testing"
)

func TestParseLocal(t *testing.T) {
	// pgx fills an empty host from PGHOST; keep the environment out of these cases.
	t.Setenv("PGHOST", "")
	t.Setenv("PGSERVICE", "")
	t.Setenv("PGDATABASE", "")

	const secret = "s3cret-pw"
	tests := []struct {
		name     string
		conn     string
		wantHost string // empty: must be refused
	}{
		{name: "ipv4 loopback", conn: "postgres://postgres@127.0.0.1:5433/expense_test?sslmode=disable", wantHost: "127.0.0.1"},
		{name: "localhost", conn: "postgres://postgres@localhost:5433/expense_test", wantHost: "localhost"},
		{name: "ipv6 loopback", conn: "postgres://postgres@[::1]:5433/expense_test", wantHost: "::1"},
		{name: "postgresql scheme", conn: "postgresql://postgres@127.0.0.1/expense_test", wantHost: "127.0.0.1"},
		{name: "keyword form", conn: "host=localhost port=5433 user=postgres dbname=expense_test", wantHost: "localhost"},
		{name: "local host parameter", conn: "postgres://postgres@127.0.0.1:5433/expense_test?host=localhost", wantHost: "localhost"},
		{name: "test anywhere in the name", conn: "postgres://postgres@127.0.0.1:5433/TestDB", wantHost: "127.0.0.1"},

		{name: "local but not a test database", conn: "postgres://postgres@127.0.0.1:5433/expense"},
		{name: "local port-forward to neondb", conn: "postgres://owner:" + secret + "@127.0.0.1:5432/neondb"},
		{name: "local without database name", conn: "postgres://postgres@127.0.0.1:5433"},
		{name: "keyword form not a test database", conn: "host=localhost dbname=postgres"},

		{name: "empty", conn: ""},
		{name: "blank", conn: "   "},
		{name: "neon", conn: "postgres://owner:" + secret + "@ep-cool-name-123456.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"},
		{name: "neon pooler", conn: "postgres://owner:" + secret + "@ep-cool-name-123456-pooler.ap-southeast-1.aws.neon.tech/neondb"},
		{name: "remote name", conn: "postgres://u:" + secret + "@db.example.com:5432/x_test"},
		{name: "remote ip", conn: "postgres://u:" + secret + "@10.0.0.5:5432/x_test"},
		{name: "look-alike name", conn: "postgres://u:" + secret + "@localhost.example.com/x_test"},
		{name: "several local hosts", conn: "postgres://u:" + secret + "@127.0.0.1:5433,localhost:5433/x_test"},
		{name: "local then remote", conn: "postgres://u:" + secret + "@127.0.0.1:5433,db.example.com:5432/x_test"},
		{name: "host parameter elsewhere", conn: "postgres://u:" + secret + "@127.0.0.1:5433/x_test?host=db.example.com"},
		{name: "hostaddr parameter elsewhere", conn: "postgres://u:" + secret + "@127.0.0.1:5433/x_test?hostaddr=10.0.0.5"},
		{name: "keyword form remote", conn: "host=db.example.com user=u password=" + secret + " dbname=x_test"},
		{name: "keyword form several hosts", conn: "host=127.0.0.1,db.example.com user=u dbname=x_test password=" + secret},
		{name: "unix socket", conn: "postgres://u@127.0.0.1/x_test?host=/var/run/postgresql"},
		{name: "not a connection string", conn: "postgres://u:" + secret + "@[bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseLocal(tt.conn)
			if tt.wantHost == "" {
				if err == nil {
					t.Fatalf("accepted, host %q; want refused", cfg.Host)
				}
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaks the password: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if cfg.Host != tt.wantHost {
				t.Fatalf("host = %q, want %q", cfg.Host, tt.wantHost)
			}
		})
	}
}

func TestParseLocalUsesPGHOST(t *testing.T) {
	// A string without a host takes PGHOST: the effective host is what is checked.
	t.Setenv("PGHOST", "db.example.com")
	if _, err := ParseLocal("postgres://u@/x_test"); err == nil {
		t.Fatal("accepted a remote PGHOST")
	}
}

func TestMigrationsDir(t *testing.T) {
	dir, err := MigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dir, "db/migrations") {
		t.Fatalf("MigrationsDir = %q", dir)
	}
}
