package db

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
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

	const merchant, badID = "Secret Merchant 4711", "not-a-uuid-4711"
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
	for _, secret := range []string{merchant, "4711", user.String()} {
		if strings.Contains(log, secret) {
			t.Errorf("log contains the value %q:\n%s", secret, log)
		}
	}
}

func TestValueFree(t *testing.T) {
	if valueFree(nil) != nil {
		t.Error("nil error changed")
	}
	if err := context.Canceled; valueFree(err) != err { //nolint:errorlint // identity on purpose
		t.Error("non-Postgres error changed")
	}
}
