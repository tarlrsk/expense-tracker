package db

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// syncBuffer is a bytes.Buffer safe for the logger and the test to share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// GORM's log lines carry the SQL with placeholders, never a parameter value, also when the
// statement fails with a Postgres message that quotes the value.
func TestLogHasNoValues(t *testing.T) {
	var out syncBuffer
	d := testDB(t, func(c *Config) {
		c.Logger = slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))
		c.SlowThreshold = time.Nanosecond // every statement is "slow", so every one is logged
	})
	user := newUser(t, d)

	const merchant, badID = "Secret Merchant qxvz", "not-a-uuid-qxvz"
	err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
		c, err := UserConn(ctx)
		if err != nil {
			return err
		}
		var echoed string
		if err := c.Raw("select ?::text", merchant).Scan(&echoed).Error; err != nil {
			return err
		}
		if echoed != merchant {
			t.Errorf("echoed %q", echoed)
		}
		return c.Exec("select ?::uuid", badID).Error
	})
	if sqlState(err) != "22P02" {
		t.Fatalf("error = %v, want invalid input syntax (22P02)", err)
	}

	log := out.String()
	for _, want := range []string{"select $1::text", "select $1::uuid", "SQLSTATE 22P02", `"level":"WARN"`, `"level":"ERROR"`} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
	for _, secret := range []string{merchant, "qxvz", user.String()} {
		if strings.Contains(log, secret) {
			t.Errorf("log contains the value %q:\n%s", secret, log)
		}
	}
}

// A scan error quotes the stored value; ErrRolledBack after a swallowed error used to quote the
// Postgres message. Neither the log nor the error WithUserTx returns may contain the value.
func TestScanAndRollbackErrorsHaveNoValues(t *testing.T) {
	var out syncBuffer
	d := testDB(t, func(c *Config) {
		c.Logger = slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	user := newUser(t, d)

	tests := []struct {
		name, secret string
		inner        func(ctx context.Context) error
		wantCause    string
	}{
		{
			name: "scan error", secret: "9876.54",
			inner: func(ctx context.Context) error {
				c, err := UserConn(ctx)
				if err != nil {
					return err
				}
				var rows []struct{ V int64 }
				return c.Raw("select ?::numeric as v", "9876.54").Scan(&rows).Error
			},
			wantCause: "cause: *fmt.wrapError",
		},
		{
			name: "postgres error", secret: "SECRET-4711",
			inner: func(ctx context.Context) error {
				c, err := UserConn(ctx)
				if err != nil {
					return err
				}
				return c.Exec("select ?::uuid", "SECRET-4711").Error
			},
			wantCause: "cause: SQLSTATE 22P02",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var innerErr error
			err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
				innerErr = d.WithUserTx(ctx, user, tt.inner) // joined; the error is swallowed below
				return nil
			})
			if innerErr == nil || !strings.Contains(innerErr.Error(), tt.secret) {
				t.Fatalf("inner error = %v; the case must produce an error quoting %q", innerErr, tt.secret)
			}
			if !errors.Is(err, tx.ErrRolledBack) || !strings.Contains(err.Error(), tt.wantCause) {
				t.Errorf("error = %v, want ErrRolledBack with %q", err, tt.wantCause)
			}
			if strings.Contains(err.Error(), tt.secret) {
				t.Errorf("returned error contains the value %q: %v", tt.secret, err)
			}
			if log := out.String(); strings.Contains(log, tt.secret) {
				t.Errorf("log contains the value %q:\n%s", tt.secret, log)
			}
		})
	}
	if log := out.String(); !strings.Contains(log, "error text hidden") || !strings.Contains(log, "*fmt.wrapError") {
		t.Errorf("the scan error was not logged by its type:\n%s", log)
	}
}

// valueFree is an allowlist: only errors known to carry no row value keep their text.
func TestValueFree(t *testing.T) {
	const secret = "SECRET-77"
	connectErr := &pgconn.ConnectError{}
	netErr := &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}
	tests := []struct {
		name string
		in   error
		want error  // the exact error returned, when set
		text string // otherwise: the returned text must contain this
	}{
		{name: "nil", in: nil, want: nil},
		{name: "cancelled", in: fmt.Errorf("query %s: %w", secret, context.Canceled), want: context.Canceled},
		{name: "deadline", in: fmt.Errorf("%s: %w", secret, context.DeadlineExceeded), want: context.DeadlineExceeded},
		{name: "record not found", in: gorm.ErrRecordNotFound, want: gorm.ErrRecordNotFound},
		{name: "translated duplicate", in: gorm.ErrDuplicatedKey, want: gorm.ErrDuplicatedKey},
		{name: "tx done", in: fmt.Errorf("%s: %w", secret, sql.ErrTxDone), want: sql.ErrTxDone},
		{name: "no rows", in: sql.ErrNoRows, want: sql.ErrNoRows},
		{name: "bad conn", in: driver.ErrBadConn, want: driver.ErrBadConn},
		{name: "eof", in: fmt.Errorf("receive message: %w", io.ErrUnexpectedEOF), want: io.ErrUnexpectedEOF},
		{name: "connect error", in: fmt.Errorf("%s: %w", secret, connectErr), want: connectErr},
		{name: "network error", in: fmt.Errorf("%s: %w", secret, netErr), want: netErr},
		{
			name: "postgres data error", text: "SQLSTATE 22P02",
			in: &pgconn.PgError{Code: "22P02", Message: `invalid input syntax for type uuid: "` + secret + `"`},
		},
		{
			name: "postgres syntax error keeps its message", text: "SQLSTATE 42601: syntax error",
			in: &pgconn.PgError{Code: "42601", Message: "syntax error"},
		},
		{
			name: "scan error", text: "error text hidden, it may quote a value: *fmt.wrapError",
			in: fmt.Errorf("sql: Scan error on column index 0: %w",
				fmt.Errorf(`converting driver.Value type string (%q) to a int64: %w`, secret, strconv.ErrSyntax)),
		},
		{name: "plain error", text: "*errors.errorString", in: errors.New("encode " + secret)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueFree(tt.in)
			if tt.text == "" {
				if got != tt.want { //nolint:errorlint // identity on purpose
					t.Errorf("valueFree = %v, want %v itself", got, tt.want)
				}
				return
			}
			if got == nil || !strings.Contains(got.Error(), tt.text) {
				t.Errorf("valueFree = %v, want it to contain %q", got, tt.text)
			}
			if got != nil && strings.Contains(got.Error(), secret) {
				t.Errorf("valueFree kept the value: %v", got)
			}
		})
	}
}
