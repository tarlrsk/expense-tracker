package trend

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

// trendSQL sums the spending per month in spent, then left-joins it to every month of the series,
// so a month without spending is a zero row. It reads categories, a table of an earlier module
// (ADR-0032 lets an adaptor read it), joined owner-scoped. `t.owner_id = ?` is the scope and the
// query's one ownership condition, so a group scope later changes only that clause
// (docs/05-roadmap.md §Groups). The months are one half-open range on occurred_on, which
// transactions_owner_date_idx serves; the series and the month of a row are plain timestamps (no
// time zone: occurred_on is a date). The sums come back as whole hundredths, never as a float.
// There is no currency filter: only THB exists (ADR-0041).
const trendSQL = `
with spent as (
  select date_trunc('month', t.occurred_on::timestamp) as first, sum(t.amount) as total
  from transactions t
  join categories c on c.owner_id = t.owner_id and c.id = t.category_id
  where t.owner_id = ?
    and t.occurred_on >= ?::date and t.occurred_on < ?::date
    and c.kind = 'expense'
  group by 1
)
select to_char(m.first, 'YYYY-MM') as month, (coalesce(s.total, 0) * 100)::bigint as spent
from generate_series(?::timestamp, ?::timestamp, interval '1 month') as m(first)
left join spent s on s.first = m.first
order by m.first`

var (
	// errNoMonths: Trend was called with months below 1.
	errNoMonths = errors.New("spending trend: months must be at least 1")
	// errNoMonth: Trend was called without a last month.
	errNoMonth = errors.New("spending trend: the month is not set")
)

type row struct {
	Month string `gorm:"column:month"`
	Spent int64  `gorm:"column:spent"`
}

func (pg) Trend(ctx context.Context, scope domain.Scope, last domain.Month, months int) ([]domain.MonthSpending, error) {
	if months < 1 {
		return nil, errNoMonths
	}
	if last.IsZero() {
		return nil, errNoMonth
	}
	first := last.AddMonths(-(months - 1))
	c, err := db.UserConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("spending trend: %w", err)
	}
	var rows []row
	res := c.Raw(trendSQL, scope.OwnerID, first.FirstDay(), last.NextFirstDay(), first.FirstDay(), last.FirstDay()).Scan(&rows)
	if err := db.Err(res); err != nil {
		return nil, fmt.Errorf("spending trend: %w", err)
	}
	// The series always has one row per month; the months are checked against the ones asked
	// for, so a fault in the query cannot shift a total onto the wrong month.
	if len(rows) != months {
		return nil, fmt.Errorf("spending trend: got %d months, want %d", len(rows), months)
	}
	out := make([]domain.MonthSpending, 0, months)
	for i, r := range rows {
		m := first.AddMonths(i)
		if r.Month != m.String() {
			return nil, fmt.Errorf("spending trend: row %d is not the month %s", i, m)
		}
		out = append(out, domain.MonthSpending{Month: m, Spent: domain.Money(r.Spent)})
	}
	return out, nil
}
