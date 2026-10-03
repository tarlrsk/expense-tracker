package update

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// The set clauses are fixed texts chosen below, never client input. updated_at is set by the
// table's trigger.
const (
	setName     = "name = ?"
	setIcon     = "icon = ?"
	setArchived = "archived = ?"
	setToEnd    = "sort_order = (select coalesce(max(e.sort_order), 0) + 1 from categories e where e.owner_id = ?)"
	returning   = " where owner_id = ? and id = ? returning id, name, icon, kind, archived, sort_order, created_at, updated_at"
)

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

func (pg) Update(ctx context.Context, ownerID, id uuid.UUID, ch Changes) (domain.Category, bool, error) {
	var (
		sets []string
		args []any
	)
	if ch.Name != nil {
		sets, args = append(sets, setName), append(args, *ch.Name)
	}
	if ch.Icon != nil {
		sets, args = append(sets, setIcon), append(args, *ch.Icon)
	}
	if ch.Archived != nil {
		sets, args = append(sets, setArchived), append(args, *ch.Archived)
	}
	if ch.ToEnd {
		sets, args = append(sets, setToEnd), append(args, ownerID)
	}
	if len(sets) == 0 {
		return domain.Category{}, false, ErrNoChange
	}
	args = append(args, ownerID, id)

	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Category{}, false, fmt.Errorf("update category: %w", err)
	}
	var r row
	res := c.Raw("update categories set "+strings.Join(sets, ", ")+returning, args...).Scan(&r)
	if err := db.Err(res); err != nil {
		if nameTaken(err) {
			return domain.Category{}, false, ErrNameTaken
		}
		return domain.Category{}, false, fmt.Errorf("update category: %w", err)
	}
	if res.RowsAffected == 0 {
		return domain.Category{}, false, nil
	}
	return domain.Category{
		ID: r.ID, Name: r.Name, Icon: r.Icon, Kind: domain.Kind(r.Kind), Archived: r.Archived,
		SortOrder: r.SortOrder, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, true, nil
}

// nameTaken reports whether err is a unique violation of the name index. A Postgres error names
// its constraint; should GORM ever hand over only its translated "duplicated key", the name index
// is still the only unique key an update of these columns can break.
func nameTaken(err error) bool {
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		return false
	}
	var pgErr *db.PgError
	return !errors.As(err, &pgErr) || pgErr.Constraint == nameIndex
}
