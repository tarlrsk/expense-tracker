// Package update changes a transaction's amount, date, category, merchant, note or currency
// (PATCH /api/transactions/{id}, ADR-0071). The id, the owner and the source never change.
package update

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Processor updates transactions.
type Processor interface {
	// Execute applies the fields that are set and returns the transaction after the change, or
	// invalid_input (no field, a broken rule, or a new category that is not one of the caller's
	// active ones) or not_found (no such transaction of the caller).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller, the transaction and the fields to change as sent; a nil field is left
// as it is.
type Request struct {
	UserID     uuid.UUID
	ID         uuid.UUID
	Amount     *string
	OccurredOn *string
	CategoryID *string
	Merchant   *string
	Note       *string
	Currency   *string
}

// Response is the transaction after the change.
type Response struct {
	Transaction domain.Transaction
}
