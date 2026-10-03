package countoperators

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

func (pg) Count(ctx context.Context) (int, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return 0, fmt.Errorf("count operators: %w", err)
	}
	var n int
	if err := db.Err(c.Raw(`select count(*) from profiles where role = ?`, string(domain.RoleOperator)).Scan(&n)); err != nil {
		return 0, fmt.Errorf("count operators: %w", err)
	}
	return n, nil
}
