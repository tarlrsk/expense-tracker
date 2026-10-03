package find

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

const findSQL = `
select id, name, icon, kind, archived, sort_order, created_at, updated_at
from categories
where owner_id = ? and id = ?`

type row struct {
	ID        uuid.UUID `gorm:"column:id"`
	Name      string    `gorm:"column:name"`
	Icon      string    `gorm:"column:icon"`
	Kind      string    `gorm:"column:kind"`
	Archived  bool      `gorm:"column:archived"`
	SortOrder int       `gorm:"column:sort_order"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (pg) Find(ctx context.Context, ownerID, id uuid.UUID) (domain.Category, bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Category{}, false, fmt.Errorf("find category: %w", err)
	}
	var r row
	res := c.Raw(findSQL, ownerID, id).Scan(&r)
	if err := db.Err(res); err != nil {
		return domain.Category{}, false, fmt.Errorf("find category: %w", err)
	}
	if res.RowsAffected == 0 {
		return domain.Category{}, false, nil
	}
	return domain.Category{
		ID: r.ID, Name: r.Name, Icon: r.Icon, Kind: domain.Kind(r.Kind), Archived: r.Archived,
		SortOrder: r.SortOrder, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, true, nil
}
