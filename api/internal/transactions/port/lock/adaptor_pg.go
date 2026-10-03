package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// for update needs the update right on a column, which app_user has (migration 0004). The amount
// and the date are read as text, so they reach Go exactly.
const lockSQL = `
select id, owner_id, amount::text as amount, currency, to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
       merchant, category_id, note, source, created_at, updated_at
from transactions
where owner_id = ? and id = ?
for update`

type row struct {
	ID         uuid.UUID `gorm:"column:id"`
	OwnerID    uuid.UUID `gorm:"column:owner_id"`
	Amount     string    `gorm:"column:amount"`
	Currency   string    `gorm:"column:currency"`
	OccurredOn string    `gorm:"column:occurred_on"`
	Merchant   string    `gorm:"column:merchant"`
	CategoryID uuid.UUID `gorm:"column:category_id"`
	Note       string    `gorm:"column:note"`
	Source     string    `gorm:"column:source"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (pg) Lock(ctx context.Context, ownerID, id uuid.UUID) (domain.Transaction, bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Transaction{}, false, fmt.Errorf("lock transaction: %w", err)
	}
	var r row
	res := c.Raw(lockSQL, ownerID, id).Scan(&r)
	if err := db.Err(res); err != nil {
		return domain.Transaction{}, false, fmt.Errorf("lock transaction: %w", err)
	}
	if res.RowsAffected == 0 {
		return domain.Transaction{}, false, nil
	}
	t, err := r.transaction()
	if err != nil {
		return domain.Transaction{}, false, fmt.Errorf("lock transaction: %w", err)
	}
	return t, true, nil
}

// transaction converts a stored row. The amount and the date always pass their rules (the
// column types and checks guarantee it); the error never quotes them, nor wraps the rule's
// invalid_input error, which would turn a stored-data fault into a 400.
func (r row) transaction() (domain.Transaction, error) {
	amount, err := domain.ParseAmount(r.Amount)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("stored amount of %s is not an amount", r.ID)
	}
	date, ok := domain.ParseDate(r.OccurredOn)
	if !ok {
		return domain.Transaction{}, fmt.Errorf("stored occurred_on of %s is not a date", r.ID)
	}
	return domain.Transaction{
		ID: r.ID, OwnerID: r.OwnerID, Amount: amount, Currency: r.Currency, OccurredOn: date,
		Merchant: r.Merchant, CategoryID: r.CategoryID, Note: r.Note, Source: domain.Source(r.Source),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}
