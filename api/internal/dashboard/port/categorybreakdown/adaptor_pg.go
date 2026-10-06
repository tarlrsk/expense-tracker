package categorybreakdown

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// breakdownSQL reads categories, a table of an earlier module (ADR-0032 lets an adaptor read it),
// joined owner-scoped. `t.owner_id = ?` is the scope and the query's one ownership condition, so
// a group scope later changes only that clause (docs/05-roadmap.md §Groups). The month is a
// half-open range on occurred_on, which transactions_owner_date_idx serves. The sums come back as
// whole hundredths, never as a float. There is no currency filter: only THB exists (ADR-0041).
// c.id is the primary key, so grouping by it allows the category's other columns.
const breakdownSQL = `
select c.id as category_id, c.name, c.icon, c.archived,
       (sum(t.amount) * 100)::bigint as total, count(*) as count
from transactions t
join categories c on c.owner_id = t.owner_id and c.id = t.category_id
where t.owner_id = ?
  and t.occurred_on >= ?::date and t.occurred_on < ?::date
  and c.kind = 'expense'
group by c.id
order by total desc, c.sort_order, c.id`

// errNoMonth: Breakdown was called without a month.
var errNoMonth = errors.New("category breakdown: the month is not set")

type row struct {
	CategoryID uuid.UUID `gorm:"column:category_id"`
	Name       string    `gorm:"column:name"`
	Icon       string    `gorm:"column:icon"`
	Archived   bool      `gorm:"column:archived"`
	Total      int64     `gorm:"column:total"`
	Count      int64     `gorm:"column:count"`
}

func (pg) Breakdown(ctx context.Context, scope domain.Scope, month domain.Month) ([]domain.CategoryTotal, error) {
	if month.IsZero() {
		return nil, errNoMonth
	}
	c, err := db.UserConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("category breakdown: %w", err)
	}
	var rows []row
	if err := db.Err(c.Raw(breakdownSQL, scope.OwnerID, month.FirstDay(), month.NextFirstDay()).Scan(&rows)); err != nil {
		return nil, fmt.Errorf("category breakdown: %w", err)
	}
	out := make([]domain.CategoryTotal, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.CategoryTotal{
			CategoryID: r.CategoryID, Name: r.Name, Icon: r.Icon, Archived: r.Archived,
			Total: domain.Money(r.Total), Count: int(r.Count),
		})
	}
	return out, nil
}
