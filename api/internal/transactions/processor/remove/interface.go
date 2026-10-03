// Package remove deletes one of the caller's transactions (DELETE /api/transactions/{id},
// ADR-0071).
package remove

import (
	"context"

	"github.com/google/uuid"
)

// Processor deletes transactions.
type Processor interface {
	// Execute deletes the transaction, or returns not_found (no such transaction of the
	// caller, also when it was already deleted).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the transaction.
type Request struct {
	UserID uuid.UUID
	ID     uuid.UUID
}

// Response is empty: a deleted transaction has nothing to return.
type Response struct{}
