package touchsession

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const touchSQL = `update sessions set last_used_at = ?, expires_at = ? where id = ? and last_used_at < ?`

func (pg) Touch(ctx context.Context, t Touch) error {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	if err := db.Err(c.Exec(touchSQL, t.Now, t.ExpiresAt, t.SessionID, t.StaleBefore)); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}
