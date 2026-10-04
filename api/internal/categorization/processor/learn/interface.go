// Package learn writes the caller's merchant rule from a saved transaction's merchant and
// category: the latest choice wins (ADR-0076, ADR-0078). The orchestrators that save
// transactions decide when a save teaches a rule; this use case decides whether the merchant and
// category can make one.
package learn

import (
	"context"

	"github.com/google/uuid"
)

// Processor learns merchant rules.
type Processor interface {
	// Execute writes the rule for the merchant's key with the merchant as written and the
	// category, replacing the caller's rule for that key. It writes nothing (Learnt false, no
	// error) when the merchant has no key (domain.MerchantKey) or is longer than
	// domain.MaxLength, or when the category is not one of the caller's non-archived ones. It
	// joins the caller's open user transaction, so the rule commits or rolls back with the save.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the merchant and category of the transaction they saved.
type Request struct {
	UserID     uuid.UUID
	Merchant   string
	CategoryID uuid.UUID
}

// Response says whether a rule was written.
type Response struct {
	Learnt bool
}
