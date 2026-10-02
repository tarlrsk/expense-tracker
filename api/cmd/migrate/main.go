// Command migrate applies the SQL migrations in db/migrations to the database in DATABASE_URL
// (ADR-0027). It reads no other setting and never prints the connection string or its password.
//
//	migrate [-dir ../db/migrations] up|down|status
//
// up applies every pending migration, down rolls back the latest one only, status lists them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/tarlrsk/expense-tracker/api/internal/db/migrate"
)

const (
	defaultDir = "../db/migrations"
	// changeTimeout covers up and down, including a cold Neon start and waiting for the
	// migration lock; statusTimeout covers status.
	changeTimeout = 2 * time.Minute
	statusTimeout = 30 * time.Second
)

const usage = "usage: migrate [-dir DIR] up|down|status"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is main without the process: it returns the exit code.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", defaultDir, "directory with the SQL migration files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	command := fs.Arg(0)
	timeout := changeTimeout
	switch command {
	case "up", "down":
	case "status":
		timeout = statusTimeout
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n%s\n", command, usage)
		return 2
	}

	dsn := getenv("DATABASE_URL")
	if dsn == "" {
		_, _ = fmt.Fprintln(stderr, "migrate: DATABASE_URL is empty; set it in .env and use the make targets, or export it")
		return 1
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		// The parser's message may quote parts of the string, so it is not shown.
		_, _ = fmt.Fprintln(stderr, "migrate: DATABASE_URL is not a valid Postgres connection string")
		return 1
	}
	if host, ok := poolerHost(cfg); ok {
		_, _ = fmt.Fprintf(stderr, "migrate: host %s is a connection pooler; use the direct host (without -pooler) "+
			"for migrations (ADR-0026): the migration lock does not work through PgBouncer\n", host)
		return 1
	}
	// Anything printed from here on is scrubbed of the password and the string itself.
	fail := func(err error) int {
		_, _ = fmt.Fprintln(stderr, "migrate:", scrub(err.Error(), dsn, cfg.Password))
		return 1
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	db := stdlib.OpenDB(*cfg)
	defer func() { _ = db.Close() }()
	_, _ = fmt.Fprintf(stdout, "database %s on %s\n", cfg.Database, cfg.Host)
	if err := db.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("connect: %w", err))
	}

	m, err := migrate.New(db, *dir)
	if err != nil {
		return fail(err)
	}

	switch command {
	case "up":
		rs, err := m.Up(ctx)
		for _, r := range rs {
			_, _ = fmt.Fprintf(stdout, "applied     %s (%s)\n", r.Name, r.Duration.Round(time.Millisecond))
		}
		if err != nil {
			return fail(err)
		}
		if len(rs) == 0 {
			_, _ = fmt.Fprintln(stdout, "up to date, nothing to apply")
		}
	case "down":
		r, err := m.Down(ctx)
		if errors.Is(err, migrate.ErrNothingToRollBack) {
			_, _ = fmt.Fprintln(stdout, "nothing to roll back")
			return 0
		}
		if err != nil {
			return fail(err)
		}
		_, _ = fmt.Fprintf(stdout, "rolled back %s (%s)\n", r.Name, r.Duration.Round(time.Millisecond))
	case "status":
		ss, err := m.Status(ctx)
		if err != nil {
			return fail(err)
		}
		for _, s := range ss {
			state := "pending"
			if s.Applied {
				state = "applied " + s.AppliedAt.UTC().Format(time.RFC3339)
			}
			_, _ = fmt.Fprintf(stdout, "%-30s %s\n", s.Name, state)
		}
	}
	return 0
}

// poolerHost returns the first host that is a Neon connection pooler (its name contains -pooler).
func poolerHost(cfg *pgx.ConnConfig) (string, bool) {
	hosts := []string{cfg.Host}
	for _, fb := range cfg.Fallbacks {
		hosts = append(hosts, fb.Host)
	}
	for _, h := range hosts {
		if strings.Contains(strings.ToLower(h), "-pooler") {
			return h, true
		}
	}
	return "", false
}

// scrub removes the connection string and the password from msg.
func scrub(msg, dsn, password string) string {
	msg = strings.ReplaceAll(msg, dsn, "[DATABASE_URL]")
	if password != "" {
		msg = strings.ReplaceAll(msg, password, "[password]")
	}
	return msg
}
