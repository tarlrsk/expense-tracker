// Package list is the query that reads one page of a scope's transactions for the export (M8,
// ADR-0087, ADR-0092), with each row's category name and kind. It runs as app_user inside
// WithUserTx, so row-level security limits it to the caller's rows; the scope is its own
// ownership condition (docs/05-roadmap.md §Groups).
package list

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/export/domain"
	txdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Filter is one page: the period's ends (nil is open; both included), the position to start
// after (nil for the first page) and the page size.
type Filter struct {
	From, To *txdomain.Date
	After    *domain.Cursor
	Limit    int
}

// Port reads export rows.
type Port interface {
	// List returns at most f.Limit of scope's transactions with occurred_on in the period, after
	// f.After, oldest first: by occurred_on, then id, both ascending. Every transaction is
	// included, whatever its category (archived ones too) or source. f.Limit must be at least 1.
	// No rows give an empty, non-nil slice.
	List(ctx context.Context, scope domain.Scope, f Filter) ([]domain.Row, error)
}
