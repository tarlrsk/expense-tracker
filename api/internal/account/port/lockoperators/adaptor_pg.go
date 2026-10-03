package lockoperators

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

// Key is the advisory-lock key ("oper" in ASCII). Its high 32 bits are zero, so in pg_locks the
// lock shows as classid 0, objid Key, objsubid 1.
const Key int64 = 0x6f706572

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

func (pg) Lock(ctx context.Context) error {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return fmt.Errorf("lock operators: %w", err)
	}
	// pg_advisory_xact_lock: released at commit or rollback (never pg_advisory_lock, which would
	// stay on the pooled connection).
	if err := db.Err(c.Exec(`select pg_advisory_xact_lock(?)`, Key)); err != nil {
		return fmt.Errorf("lock operators: %w", err)
	}
	return nil
}
