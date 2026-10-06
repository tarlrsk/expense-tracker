// Package trend is the query that sums a scope's spending per month over several months, for the
// dashboard (M6, ADR-0088). It runs as app_user inside WithUserTx, so row-level security limits
// it to the caller's rows; the scope is its own ownership condition (docs/05-roadmap.md §Groups).
// The sums are made in SQL; no transaction row leaves the adaptor.
package trend

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
)

// Port sums spending per month.
type Port interface {
	// Trend returns the spending (expense-kind categories, archived ones included) of scope in
	// last and the months-1 months before it, oldest first: always exactly months rows, zero for
	// a month without spending. months must be at least 1 (domain.TrendMonths on the dashboard).
	Trend(ctx context.Context, scope domain.Scope, last domain.Month, months int) ([]domain.MonthSpending, error)
}
