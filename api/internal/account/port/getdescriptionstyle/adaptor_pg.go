package getdescriptionstyle

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

const getSQL = `select description_style from profiles where id = ?`

type row struct {
	DescriptionStyle string `gorm:"column:description_style"`
}

func (pg) Get(ctx context.Context, userID uuid.UUID) (domain.DescriptionStyle, bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return "", false, fmt.Errorf("get description style: %w", err)
	}
	var r row
	res := c.Raw(getSQL, userID).Scan(&r)
	if err := db.Err(res); err != nil {
		return "", false, fmt.Errorf("get description style: %w", err)
	}
	if res.RowsAffected == 0 {
		return "", false, nil
	}
	return domain.DescriptionStyle(r.DescriptionStyle), true, nil
}
