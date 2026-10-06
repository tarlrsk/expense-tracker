// Package categorybreakdown is the query that sums a scope's spending per category in one month,
// for the dashboard (M6, ADR-0088). It runs as app_user inside WithUserTx, so row-level security
// limits it to the caller's rows; the scope is its own ownership condition (docs/05-roadmap.md
// §Groups). The sums are made in SQL; no transaction row leaves the adaptor.
package categorybreakdown

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
)

// Port breaks a month's spending down by category.
type Port interface {
	// Breakdown returns one row per expense category of scope that has transactions with
	// occurred_on in month, archived categories included (flagged). Income categories are never
	// listed. Rows are ordered by total, largest first, then by the category's sort_order, then
	// by id. A month without spending gives an empty, non-nil slice.
	Breakdown(ctx context.Context, scope domain.Scope, month domain.Month) ([]domain.CategoryTotal, error)
}
