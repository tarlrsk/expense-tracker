// Package updatetransaction is PATCH /api/transactions/{id} with merchant-rule learning: it
// updates the transaction and, when that changed its category or merchant and left it with a
// merchant, learns the merchant's rule in the same user transaction (ADR-0071, ADR-0076,
// ADR-0078). It lives in categorization, the latest module it touches (ADR-0032).
package updatetransaction

import (
	"context"

	transactionsupdateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/update"
)

// Orchestrator updates transactions and learns from them. It has the method of the transactions
// update processor, so the transactions handler takes it unchanged.
type Orchestrator interface {
	// Execute answers exactly like the transactions update processor: the same response, and
	// its errors unchanged (invalid_input or not_found with the same message). A failure to
	// learn the rule is an internal error, and then nothing is saved.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the transactions update processor's request.
type Request = transactionsupdateproc.Request

// Response is the transactions update processor's response.
type Response = transactionsupdateproc.Response
