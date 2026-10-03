// Package list returns one page of the caller's transactions, newest first, filtered by period,
// category and source (GET /api/transactions, ADR-0071).
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Processor lists transactions.
type Processor interface {
	// Execute returns one page ordered by occurred_on descending, then id descending, and the
	// cursor of the next page while more rows exist; invalid_input for a malformed parameter.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the query parameters as sent; a nil parameter was not sent.
type Request struct {
	UserID   uuid.UUID
	Month    *string
	From     *string
	To       *string
	Category *string
	Source   *string
	Limit    *string
	Cursor   *string
}

// Response is the page and, while more rows exist, the cursor that continues after it.
type Response struct {
	Transactions []domain.Transaction
	Next         *domain.Cursor
}
