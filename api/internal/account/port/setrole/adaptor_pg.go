package setrole

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const setSQL = `update profiles set role = ? where id = ? and role <> ?`

func (pg) Set(ctx context.Context, userID uuid.UUID, role domain.Role) (bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return false, fmt.Errorf("set role: %w", err)
	}
	res := c.Exec(setSQL, string(role), userID, string(role))
	if err := db.Err(res); err != nil {
		return false, fmt.Errorf("set role: %w", err)
	}
	return res.RowsAffected > 0, nil
}
