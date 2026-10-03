package insert

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// categoryKey is the owner-scoped foreign key to categories (migration 0004).
const categoryKey = "transactions_category_fk"

// The amount and the date are sent as text and cast by Postgres, so they are stored exactly.
// on conflict (id) do nothing: an id that exists, also one of another user that row-level
// security hides, inserts nothing and returns no row, without an error that would end the
// transaction. A concurrent insert of the same id waits for the first to commit, then does the
// same.
const insertSQL = `
insert into transactions (id, owner_id, amount, currency, occurred_on, merchant, category_id, note, source)
values (?, ?, ?::numeric, ?, ?::date, ?, ?, ?, ?)
on conflict (id) do nothing
returning id, owner_id, amount::text as amount, currency, to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
          merchant, category_id, note, source, created_at, updated_at`

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

func (pg) Insert(ctx context.Context, nt NewTransaction) (domain.Transaction, bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Transaction{}, false, fmt.Errorf("insert transaction: %w", err)
	}
	var r row
	res := c.Raw(insertSQL, nt.ID, nt.OwnerID, nt.Amount.String(), nt.Currency, nt.OccurredOn.String(),
		nt.Merchant, nt.CategoryID, nt.Note, string(nt.Source)).Scan(&r)
	if err := db.Err(res); err != nil {
		if categoryRefused(err) {
			return domain.Transaction{}, false, ErrCategory
		}
		return domain.Transaction{}, false, fmt.Errorf("insert transaction: %w", err)
	}
	if res.RowsAffected == 0 {
		return domain.Transaction{}, false, nil
	}
	t, err := r.transaction()
	if err != nil {
		return domain.Transaction{}, false, fmt.Errorf("insert transaction: %w", err)
	}
	return t, true, nil
}

// categoryRefused reports whether err is a foreign-key violation of the category key. Should GORM
// ever hand over only its translated error, the category key is still the only foreign key this
// insert can break with an owner that exists (the caller).
func categoryRefused(err error) bool {
	if !errors.Is(err, gorm.ErrForeignKeyViolated) {
		return false
	}
	var pgErr *db.PgError
	return !errors.As(err, &pgErr) || pgErr.Constraint == categoryKey
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
