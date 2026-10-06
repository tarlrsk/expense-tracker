// Package topdescriptions is the query that lists the descriptions a scope spent most under in
// one month, for the dashboard (M6, ADR-0088). The description is the transaction's merchant
// column. It runs as app_user inside WithUserTx, so row-level security limits it to the caller's
// rows; the scope is its own ownership condition (docs/05-roadmap.md §Groups). The sums are made
// in SQL; no transaction row leaves the adaptor.
package topdescriptions

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
)

// Port lists the top descriptions.
type Port interface {
	// Top returns at most limit rows of scope's spending (expense-kind categories, archived ones
	// included) with occurred_on in month, one per description. Descriptions are grouped ignoring
	// upper and lower case and the spaces at their ends ("Grab", "grab" and " GRAB " are one row;
	// "7-Eleven" and "7 eleven" are two); a description that is empty after trimming is left out.
	// The name shown for a group is the trimmed spelling used by the most of its transactions that
	// month; on a tie the one used most recently, then the first in byte order. Rows are ordered
	// by total, largest first, then by count, largest first, then by the grouping key in byte
	// order. limit must be at least 1 (domain.TopDescriptionsLimit on the dashboard). A month
	// without such spending gives an empty, non-nil slice.
	Top(ctx context.Context, scope domain.Scope, month domain.Month, limit int) ([]domain.DescriptionTotal, error)
}
