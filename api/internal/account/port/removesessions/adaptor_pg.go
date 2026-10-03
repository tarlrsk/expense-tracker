package removesessions

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

func (pg) Remove(ctx context.Context, userID, keep uuid.UUID) error {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return fmt.Errorf("remove sessions: %w", err)
	}
	if err := db.Err(c.Exec(`delete from sessions where user_id = ? and id <> ?`, userID, keep)); err != nil {
		return fmt.Errorf("remove sessions: %w", err)
	}
	return nil
}
