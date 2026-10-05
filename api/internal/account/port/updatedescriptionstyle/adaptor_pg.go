package updatedescriptionstyle

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

func (pg) Update(ctx context.Context, userID uuid.UUID, style domain.DescriptionStyle) (bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return false, fmt.Errorf("update description style: %w", err)
	}
	res := c.Exec(`update profiles set description_style = ? where id = ?`, string(style), userID)
	if err := db.Err(res); err != nil {
		return false, fmt.Errorf("update description style: %w", err)
	}
	return res.RowsAffected > 0, nil
}
