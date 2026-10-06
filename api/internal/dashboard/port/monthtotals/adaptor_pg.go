package monthtotals

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

// totalsSQL reads categories, a table of an earlier module (ADR-0032 lets an adaptor read it),
// joined owner-scoped. `t.owner_id = ?` is the scope and the query's one ownership condition, so
// a group scope later changes only that clause (docs/05-roadmap.md §Groups). The month is a
// half-open range on occurred_on, which transactions_owner_date_idx serves. The sums come back as
// whole hundredths, never as a float. There is no currency filter: only THB exists (ADR-0041).
const totalsSQL = `
select (coalesce(sum(t.amount) filter (where c.kind = 'expense'), 0) * 100)::bigint as spent,
       (coalesce(sum(t.amount) filter (where c.kind = 'income'), 0) * 100)::bigint as income
from transactions t
join categories c on c.owner_id = t.owner_id and c.id = t.category_id
where t.owner_id = ?
  and t.occurred_on >= ?::date and t.occurred_on < ?::date`

// errNoMonth: Totals was called without a month.
var errNoMonth = errors.New("month totals: the month is not set")

type row struct {
	Spent  int64 `gorm:"column:spent"`
	Income int64 `gorm:"column:income"`
}

func (pg) Totals(ctx context.Context, scope domain.Scope, month domain.Month) (domain.MonthTotals, error) {
	if month.IsZero() {
		return domain.MonthTotals{}, errNoMonth
	}
	c, err := db.UserConn(ctx)
	if err != nil {
		return domain.MonthTotals{}, fmt.Errorf("month totals: %w", err)
	}
	var r row
	if err := db.Err(c.Raw(totalsSQL, scope.OwnerID, month.FirstDay(), month.NextFirstDay()).Scan(&r)); err != nil {
		return domain.MonthTotals{}, fmt.Errorf("month totals: %w", err)
	}
	return domain.MonthTotals{Spent: domain.Money(r.Spent), Income: domain.Money(r.Income)}, nil
}
