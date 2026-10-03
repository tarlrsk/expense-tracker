package insert

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// nameIndex is the partial unique index on (owner_id, lower(name)) of non-archived categories
// (migration 0003).
const nameIndex = "categories_owner_name_idx"

// The aggregate always returns one row, so the insert always adds one row.
const insertSQL = `
insert into categories (owner_id, name, icon, kind, sort_order)
select ?::uuid, ?, ?, ?, coalesce(max(sort_order), 0) + 1
from categories
where owner_id = ?
returning id, name, icon, kind, archived, sort_order, created_at, updated_at`

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

func (pg) Insert(ctx context.Context, nc NewCategory) (domain.Category, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Category{}, fmt.Errorf("insert category: %w", err)
	}
	var r row
	if err := db.Err(c.Raw(insertSQL, nc.OwnerID, nc.Name, nc.Icon, string(nc.Kind), nc.OwnerID).Scan(&r)); err != nil {
		if nameTaken(err) {
			return domain.Category{}, ErrNameTaken
		}
		return domain.Category{}, fmt.Errorf("insert category: %w", err)
	}
	return domain.Category{
		ID: r.ID, Name: r.Name, Icon: r.Icon, Kind: domain.Kind(r.Kind), Archived: r.Archived,
		SortOrder: r.SortOrder, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}

// nameTaken reports whether err is a unique violation of the name index. A Postgres error names
// its constraint; should GORM ever hand over only its translated "duplicated key", the name index
// is still the only unique key an insert can break (the ids are generated).
func nameTaken(err error) bool {
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return false
	}
	var pgErr *db.PgError
	return !errors.As(err, &pgErr) || pgErr.Constraint == nameIndex
}
