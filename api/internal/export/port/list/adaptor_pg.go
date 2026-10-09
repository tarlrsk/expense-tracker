package list

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/export/domain"
	txdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// The where clauses are fixed texts chosen below, never client input. The query reads categories,
// a table of an earlier module (ADR-0032 lets an adaptor read it), joined owner-scoped.
// `t.owner_id = ?` is the scope and the query's one ownership condition, so a group scope later
// changes only that clause (docs/05-roadmap.md §Groups). The cursor is a row comparison in the
// order of transactions_owner_date_idx (owner_id, occurred_on desc, id desc), read backwards. The
// amount comes back as the stored decimal text, never as a float.
const (
	selectSQL = `
select t.id, to_char(t.occurred_on, 'YYYY-MM-DD') as occurred_on, t.amount::text as amount,
       c.kind, c.name as category, t.merchant, t.note, t.source
from transactions t
join categories c on c.owner_id = t.owner_id and c.id = t.category_id
where t.owner_id = ?`
	whereFrom  = " and t.occurred_on >= ?::date"
	whereTo    = " and t.occurred_on <= ?::date"
	whereAfter = " and (t.occurred_on, t.id) > (?::date, ?::uuid)"
	orderLimit = " order by t.occurred_on, t.id limit ?"
)

// errNoLimit: List was called without a positive limit.
var errNoLimit = errors.New("export rows: the limit must be at least 1")

type row struct {
	ID         uuid.UUID `gorm:"column:id"`
	OccurredOn string    `gorm:"column:occurred_on"`
	Amount     string    `gorm:"column:amount"`
	Kind       string    `gorm:"column:kind"`
	Category   string    `gorm:"column:category"`
	Merchant   string    `gorm:"column:merchant"`
	Note       string    `gorm:"column:note"`
	Source     string    `gorm:"column:source"`
}

func (pg) List(ctx context.Context, scope domain.Scope, f Filter) ([]domain.Row, error) {
	if f.Limit < 1 {
		return nil, errNoLimit
	}
	var q strings.Builder
	q.WriteString(selectSQL)
	args := []any{scope.OwnerID}
	if f.From != nil {
		q.WriteString(whereFrom)
		args = append(args, f.From.String())
	}
	if f.To != nil {
		q.WriteString(whereTo)
		args = append(args, f.To.String())
	}
	if f.After != nil {
		q.WriteString(whereAfter)
		args = append(args, f.After.OccurredOn.String(), f.After.ID)
	}
	q.WriteString(orderLimit)
	args = append(args, f.Limit)

	c, err := db.UserConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("export rows: %w", err)
	}
	var rows []row
	if err := db.Err(c.Raw(q.String(), args...).Scan(&rows)); err != nil {
		return nil, fmt.Errorf("export rows: %w", err)
	}
	out := make([]domain.Row, 0, len(rows))
	for _, r := range rows {
		// The column type guarantees a date; the error never quotes it.
		date, ok := txdomain.ParseDate(r.OccurredOn)
		if !ok {
			return nil, fmt.Errorf("export rows: stored occurred_on of %s is not a date", r.ID)
		}
		out = append(out, domain.Row{
			ID: r.ID, OccurredOn: date, Amount: r.Amount, Kind: r.Kind, Category: r.Category,
			Description: r.Merchant, Note: r.Note, Source: r.Source,
		})
	}
	return out, nil
}
