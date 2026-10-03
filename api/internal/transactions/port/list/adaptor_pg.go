package list

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// The where clauses are fixed texts chosen below, never client input. The cursor is a row
// comparison, which follows the order of the index (owner_id, occurred_on desc, id desc).
const (
	selectSQL = `
select id, owner_id, amount::text as amount, currency, to_char(occurred_on, 'YYYY-MM-DD') as occurred_on,
       merchant, category_id, note, source, created_at, updated_at
from transactions
where owner_id = ?`
	whereFrom     = " and occurred_on >= ?::date"
	whereTo       = " and occurred_on <= ?::date"
	whereCategory = " and category_id = ?"
	whereSource   = " and source = ?"
	whereAfter    = " and (occurred_on, id) < (?::date, ?::uuid)"
	orderLimit    = " order by occurred_on desc, id desc limit ?"
)

// errNoLimit: List was called without a positive limit.
var errNoLimit = errors.New("list transactions: the limit must be at least 1")

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

func (pg) List(ctx context.Context, ownerID uuid.UUID, f Filter) ([]domain.Transaction, error) {
	if f.Limit < 1 {
		return nil, errNoLimit
	}
	var q strings.Builder
	q.WriteString(selectSQL)
	args := []any{ownerID}
	if f.From != nil {
		q.WriteString(whereFrom)
		args = append(args, f.From.String())
	}
	if f.To != nil {
		q.WriteString(whereTo)
		args = append(args, f.To.String())
	}
	if f.CategoryID != nil {
		q.WriteString(whereCategory)
		args = append(args, *f.CategoryID)
	}
	if f.Source != nil {
		q.WriteString(whereSource)
		args = append(args, string(*f.Source))
	}
	if f.After != nil {
		q.WriteString(whereAfter)
		args = append(args, f.After.OccurredOn.String(), f.After.ID)
	}
	q.WriteString(orderLimit)
	args = append(args, f.Limit)

	c, err := db.UserConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	var rows []row
	if err := db.Err(c.Raw(q.String(), args...).Scan(&rows)); err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	out := make([]domain.Transaction, 0, len(rows))
	for _, r := range rows {
		t, err := r.transaction()
		if err != nil {
			return nil, fmt.Errorf("list transactions: %w", err)
		}
		out = append(out, t)
	}
	return out, nil
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
