package topdescriptions

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// topSQL keys every spending row by lower(btrim(merchant)) (ADR-0088) in spending, counts each
// trimmed spelling of a key in spellings, picks the shown spelling per key in shown, and sums per
// key in per_key. collate "C" makes the last tie-breaks byte order, whatever the database's
// collation. It reads categories, a table of an earlier module (ADR-0032 lets an adaptor read
// it), joined owner-scoped. `t.owner_id = ?` is the scope and the query's one ownership
// condition, so a group scope later changes only that clause (docs/05-roadmap.md §Groups). The
// month is a half-open range on occurred_on, which transactions_owner_date_idx serves. The sums
// come back as whole hundredths, never as a float. There is no currency filter: only THB exists
// (ADR-0041).
const topSQL = `
with spending as (
  select lower(btrim(t.merchant)) as key, btrim(t.merchant) as name, t.amount, t.occurred_on
  from transactions t
  join categories c on c.owner_id = t.owner_id and c.id = t.category_id
  where t.owner_id = ?
    and t.occurred_on >= ?::date and t.occurred_on < ?::date
    and c.kind = 'expense'
    and btrim(t.merchant) <> ''
),
spellings as (
  select key, name, count(*) as uses, max(occurred_on) as last_used
  from spending
  group by key, name
),
shown as (
  select distinct on (key) key, name
  from spellings
  order by key, uses desc, last_used desc, name collate "C"
),
per_key as (
  select key, sum(amount) as total, count(*) as count
  from spending
  group by key
)
select s.name as description, (g.total * 100)::bigint as total, g.count
from per_key g
join shown s on s.key = g.key
order by g.total desc, g.count desc, g.key collate "C"
limit ?`

var (
	// errNoLimit: Top was called without a positive limit.
	errNoLimit = errors.New("top descriptions: the limit must be at least 1")
	// errNoMonth: Top was called without a month.
	errNoMonth = errors.New("top descriptions: the month is not set")
)

type row struct {
	Description string `gorm:"column:description"`
	Total       int64  `gorm:"column:total"`
	Count       int64  `gorm:"column:count"`
}

func (pg) Top(ctx context.Context, scope domain.Scope, month domain.Month, limit int) ([]domain.DescriptionTotal, error) {
	if limit < 1 {
		return nil, errNoLimit
	}
	if month.IsZero() {
		return nil, errNoMonth
	}
	c, err := db.UserConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("top descriptions: %w", err)
	}
	var rows []row
	if err := db.Err(c.Raw(topSQL, scope.OwnerID, month.FirstDay(), month.NextFirstDay(), limit).Scan(&rows)); err != nil {
		return nil, fmt.Errorf("top descriptions: %w", err)
	}
	out := make([]domain.DescriptionTotal, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.DescriptionTotal{Description: r.Description, Total: domain.Money(r.Total), Count: int(r.Count)})
	}
	return out, nil
}
