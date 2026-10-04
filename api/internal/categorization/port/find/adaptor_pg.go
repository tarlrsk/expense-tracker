package find

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// The join reads categories, a table of an earlier module (ADR-0032 lets an adaptor read it). The
// owner-scoped foreign key (migration 0005) makes the category the rule owner's own; the owner is
// still named on both sides so the join never relies on it.
const findSQL = `
select r.id, r.merchant_key, r.merchant, r.category_id, r.created_at, r.updated_at
from merchant_rules r
join categories c on c.owner_id = r.owner_id and c.id = r.category_id
where r.owner_id = ? and r.merchant_key = ? and not c.archived`

type row struct {
	ID          uuid.UUID `gorm:"column:id"`
	MerchantKey string    `gorm:"column:merchant_key"`
	Merchant    string    `gorm:"column:merchant"`
	CategoryID  uuid.UUID `gorm:"column:category_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (pg) Find(ctx context.Context, ownerID uuid.UUID, merchantKey string) (domain.Rule, bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Rule{}, false, fmt.Errorf("find merchant rule: %w", err)
	}
	var r row
	res := c.Raw(findSQL, ownerID, merchantKey).Scan(&r)
	if err := db.Err(res); err != nil {
		return domain.Rule{}, false, fmt.Errorf("find merchant rule: %w", err)
	}
	if res.RowsAffected == 0 {
		return domain.Rule{}, false, nil
	}
	return domain.Rule{
		ID: r.ID, MerchantKey: r.MerchantKey, Merchant: r.Merchant, CategoryID: r.CategoryID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, true, nil
}
