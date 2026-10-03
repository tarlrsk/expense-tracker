// Package list is the query that reads one page of a user's transactions (GET
// /api/transactions, ADR-0071). It runs as app_user inside WithUserTx, so row-level security
// limits it to the caller's rows; it also filters by owner itself.
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Filter narrows and positions the list; a nil field does not filter.
type Filter struct {
	// From and To are the dates of the period, both included.
	From *domain.Date
	To   *domain.Date
	// CategoryID keeps one category's transactions.
	CategoryID *uuid.UUID
	// Source keeps one source's transactions.
	Source *domain.Source
	// After starts the list after this position (keyset paging, never OFFSET).
	After *domain.Cursor
	// Limit is the most rows to return; at least 1.
	Limit int
}

// Port lists transactions.
type Port interface {
	// List returns at most f.Limit transactions of ownerID that match f, ordered by occurred_on
	// descending, then id descending (index transactions_owner_date_idx).
	List(ctx context.Context, ownerID uuid.UUID, f Filter) ([]domain.Transaction, error)
}
