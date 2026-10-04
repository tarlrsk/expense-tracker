// Package create adds a transaction with the id the client made, or returns the one that already
// has that id (POST /api/transactions, ADR-0040, ADR-0041, ADR-0071).
package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Processor creates transactions.
type Processor interface {
	// Execute creates the transaction (Created true) or, when the caller already has one with
	// the id, returns it as stored and writes nothing (Created false). It returns invalid_input
	// for a broken rule, for a category that is not one of the caller's active ones, and for an
	// id another user's transaction has, which is answered like a malformed id.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the new transaction as sent; Execute applies the rules. A nil
// Currency is THB and a nil Source is manual. RawInput may be set only with Source text; nil is
// none.
type Request struct {
	UserID     uuid.UUID
	ID         string
	Amount     string
	OccurredOn string
	CategoryID string
	Merchant   string
	Note       string
	Currency   *string
	Source     *string
	RawInput   *string
}

// Response is the transaction as stored, and whether this request created it.
type Response struct {
	Transaction domain.Transaction
	Created     bool
}
