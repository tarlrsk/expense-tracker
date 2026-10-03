package count

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

func (pg) Count(ctx context.Context, ownerID uuid.UUID) (int, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return 0, fmt.Errorf("count categories: %w", err)
	}
	var n int
	if err := db.Err(c.Raw(`select count(*) from categories where owner_id = ?`, ownerID).Scan(&n)); err != nil {
		return 0, fmt.Errorf("count categories: %w", err)
	}
	return n, nil
}
