// Package createtransaction is POST /api/transactions with merchant-rule learning: it creates the
// transaction and, when that really inserted one with a merchant, learns the merchant's rule in
// the same user transaction (ADR-0040, ADR-0076, ADR-0078). It lives in categorization, the latest
// module it touches (ADR-0032).
package createtransaction

import (
	"context"

	transactionscreateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/create"
)

// Orchestrator creates transactions and learns from them. It has the method of the transactions
// create processor, so the transactions handler takes it unchanged.
type Orchestrator interface {
	// Execute answers exactly like the transactions create processor: the same response, and
	// its errors unchanged (invalid_input with the same message). A failure to learn the rule
	// is an internal error, and then nothing is saved.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the transactions create processor's request.
type Request = transactionscreateproc.Request

// Response is the transactions create processor's response.
type Response = transactionscreateproc.Response
