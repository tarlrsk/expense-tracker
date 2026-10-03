package removelinks

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

func (pg) Remove(ctx context.Context, userID uuid.UUID) error {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return fmt.Errorf("remove links: %w", err)
	}
	if err := db.Err(c.Exec(`delete from email_tokens where user_id = ? and purpose = 'set_password'`, userID)); err != nil {
		return fmt.Errorf("remove links: %w", err)
	}
	return nil
}
