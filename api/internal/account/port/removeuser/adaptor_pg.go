package removeuser

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

func (pg) Remove(ctx context.Context, userID uuid.UUID) (bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return false, fmt.Errorf("remove user: %w", err)
	}
	res := c.Exec(`delete from users where id = ?`, userID)
	if err := db.Err(res); err != nil {
		return false, fmt.Errorf("remove user: %w", err)
	}
	return res.RowsAffected > 0, nil
}
