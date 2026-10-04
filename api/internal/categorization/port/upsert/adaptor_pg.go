package upsert

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// errUnreturned: the statement returned no row, which an insert or update never does.
var errUnreturned = errors.New("upsert merchant rule: no row returned")

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// One statement (ADR-0040). The conflict target is the unique (owner_id, merchant_key) of
// migration 0005; the update sets only merchant and category_id (merchant_key never changes and
// app_user may not update it), and updated_at is set by the table's trigger.
const upsertSQL = `
insert into merchant_rules (owner_id, merchant_key, merchant, category_id)
values (?, ?, ?, ?)
on conflict (owner_id, merchant_key) do update
set merchant = excluded.merchant, category_id = excluded.category_id
returning id, merchant_key, merchant, category_id, created_at, updated_at`

type row struct {
	ID          uuid.UUID `gorm:"column:id"`
	MerchantKey string    `gorm:"column:merchant_key"`
	Merchant    string    `gorm:"column:merchant"`
	CategoryID  uuid.UUID `gorm:"column:category_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (pg) Upsert(ctx context.Context, nr NewRule) (domain.Rule, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Rule{}, fmt.Errorf("upsert merchant rule: %w", err)
	}
	var r row
	res := c.Raw(upsertSQL, nr.OwnerID, nr.MerchantKey, nr.Merchant, nr.CategoryID).Scan(&r)
	if err := db.Err(res); err != nil {
		return domain.Rule{}, fmt.Errorf("upsert merchant rule: %w", err)
	}
	if res.RowsAffected == 0 {
		return domain.Rule{}, errUnreturned
	}
	return domain.Rule{
		ID: r.ID, MerchantKey: r.MerchantKey, Merchant: r.Merchant, CategoryID: r.CategoryID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}
