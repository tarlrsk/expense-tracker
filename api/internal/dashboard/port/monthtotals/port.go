// Package monthtotals is the query that sums what a scope spent and received in one month, for
// the dashboard (M6, ADR-0088). It runs as app_user inside WithUserTx, so row-level security
// limits it to the caller's rows; the scope is its own ownership condition (docs/05-roadmap.md
// §Groups). The sums are made in SQL; no transaction row leaves the adaptor.
package monthtotals

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
)

// Port sums a month.
type Port interface {
	// Totals returns the sum of the expense-kind transactions (Spent) and of the income-kind
	// transactions (Income) of scope with occurred_on in month. The kind is the category's;
	// archived categories count like any other. A month without transactions is zeros.
	Totals(ctx context.Context, scope domain.Scope, month domain.Month) (domain.MonthTotals, error)
}
