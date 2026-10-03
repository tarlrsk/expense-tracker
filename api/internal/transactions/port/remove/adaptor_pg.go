package remove

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

const removeSQL = `delete from transactions where owner_id = ? and id = ?`

func (pg) Remove(ctx context.Context, ownerID, id uuid.UUID) (bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return false, fmt.Errorf("remove transaction: %w", err)
	}
	res := c.Exec(removeSQL, ownerID, id)
	if err := db.Err(res); err != nil {
		return false, fmt.Errorf("remove transaction: %w", err)
	}
	return res.RowsAffected > 0, nil
}
