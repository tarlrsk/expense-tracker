package getrole

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

const getSQL = `
select coalesce(p.role, ?) as role
from users u
left join profiles p on p.id = u.id
where u.id = ?`

type row struct {
	Role string `gorm:"column:role"`
}

func (pg) Get(ctx context.Context, userID uuid.UUID) (domain.Role, bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return "", false, fmt.Errorf("get role: %w", err)
	}
	var r row
	res := c.Raw(getSQL, string(domain.RoleUser), userID).Scan(&r)
	if err := db.Err(res); err != nil {
		return "", false, fmt.Errorf("get role: %w", err)
	}
	if res.RowsAffected == 0 {
		return "", false, nil
	}
	return domain.Role(r.Role), true, nil
}
