package update

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// The set clauses are fixed texts chosen below, never client input. updated_at is set by the
// table's trigger. The amount and the date are sent as text and cast by Postgres.
const (
	setAmount     = "amount = ?::numeric"
	setCurrency   = "currency = ?"
	setOccurredOn = "occurred_on = ?::date"
	setMerchant   = "merchant = ?"
	setCategoryID = "category_id = ?"
	setNote       = "note = ?"
	returning     = ` where owner_id = ? and id = ?
returning id, owner_id, amount::text as amount, currency, to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
          merchant, category_id, note, source, raw_input, created_at, updated_at`
)

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
	RawInput   string    `gorm:"column:raw_input"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (pg) Update(ctx context.Context, ownerID, id uuid.UUID, ch Changes) (domain.Transaction, bool, error) {
	var (
		sets []string
		args []any
	)
	if ch.Amount != nil {
		sets, args = append(sets, setAmount), append(args, ch.Amount.String())
	}
	if ch.Currency != nil {
		sets, args = append(sets, setCurrency), append(args, *ch.Currency)
	}
	if ch.OccurredOn != nil {
		sets, args = append(sets, setOccurredOn), append(args, ch.OccurredOn.String())
	}
	if ch.Merchant != nil {
		sets, args = append(sets, setMerchant), append(args, *ch.Merchant)
	}
	if ch.CategoryID != nil {
		sets, args = append(sets, setCategoryID), append(args, *ch.CategoryID)
	}
	if ch.Note != nil {
		sets, args = append(sets, setNote), append(args, *ch.Note)
	}
	if len(sets) == 0 {
		return domain.Transaction{}, false, ErrNoChange
	}
	args = append(args, ownerID, id)

	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.Transaction{}, false, fmt.Errorf("update transaction: %w", err)
	}
	var r row
	res := c.Raw("update transactions set "+strings.Join(sets, ", ")+returning, args...).Scan(&r)
	if err := db.Err(res); err != nil {
		if categoryRefused(err) {
			return domain.Transaction{}, false, ErrCategory
		}
		return domain.Transaction{}, false, fmt.Errorf("update transaction: %w", err)
	}
	if res.RowsAffected == 0 {
		return domain.Transaction{}, false, nil
	}
	t, err := r.transaction()
	if err != nil {
		return domain.Transaction{}, false, fmt.Errorf("update transaction: %w", err)
	}
	return t, true, nil
}

// categoryRefused reports whether err is a foreign-key violation of the category key. Should GORM
// ever hand over only its translated error, the category key is still the only foreign key an
// update of these columns can break.
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
		Merchant: r.Merchant, CategoryID: r.CategoryID, Note: r.Note, Source: domain.Source(r.Source), RawInput: r.RawInput,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}
