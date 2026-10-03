package list

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

const listSQL = `
select id, name, icon, kind, archived, sort_order, created_at, updated_at
from categories
where owner_id = ?
order by sort_order, id`

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

func (pg) List(ctx context.Context, ownerID uuid.UUID) ([]domain.Category, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	var rows []row
	if err := db.Err(c.Raw(listSQL, ownerID).Scan(&rows)); err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	out := make([]domain.Category, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Category{
			ID: r.ID, Name: r.Name, Icon: r.Icon, Kind: domain.Kind(r.Kind), Archived: r.Archived,
			SortOrder: r.SortOrder, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}
