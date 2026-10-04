// Package matchrules looks up the caller's merchant rules for the merchants the AI read, so a
// rule's category comes before the AI's (docs/03-modules.md §M5, PLAN-0003 T5). It writes
// nothing.
package matchrules

import (
	"context"

	"github.com/google/uuid"

	categorizationdomain "github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
)

// Processor matches merchants to rules.
type Processor interface {
	// Execute returns, for each merchant in the order given, the caller's rule for its key
	// (categorization/domain.MerchantKey) when there is one whose category is active (ADR-0078).
	// A merchant without a key has no rule. All lookups run in one user transaction, each key
	// once, and none is opened when no merchant has a key.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the merchants to match.
type Request struct {
	UserID    uuid.UUID
	Merchants []string
}

// Response has one Match per merchant, in the order given.
type Response struct {
	Matches []Match
}

// Match is the rule found for one merchant; Found is false when there is none.
type Match struct {
	Found bool
	Rule  categorizationdomain.Rule
}
