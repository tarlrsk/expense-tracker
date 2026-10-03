package updatedisplayname

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

func (pg) Update(ctx context.Context, userID uuid.UUID, displayName string) (bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return false, fmt.Errorf("update display name: %w", err)
	}
	res := c.Exec(`update profiles set display_name = ? where id = ?`, displayName, userID)
	if err := db.Err(res); err != nil {
		return false, fmt.Errorf("update display name: %w", err)
	}
	return res.RowsAffected > 0, nil
}
