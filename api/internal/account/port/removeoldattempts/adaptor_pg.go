package removeoldattempts

import (
	"context"
	"fmt"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

func (pg) Remove(ctx context.Context, before time.Time) error {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return fmt.Errorf("remove old login attempts: %w", err)
	}
	if err := db.Err(c.Exec(`delete from login_attempts where attempted_at < ?`, before)); err != nil {
		return fmt.Errorf("remove old login attempts: %w", err)
	}
	return nil
}
