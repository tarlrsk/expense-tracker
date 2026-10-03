// Package promote gives an existing account the operator role. Only the operator command uses it
// (ADR-0035, ADR-0056); no API endpoint sets a role and there is no demote.
package promote

import (
	"context"

	"github.com/google/uuid"
)

// Processor promotes accounts.
type Processor interface {
	// Execute makes the account with that email an operator, or returns not_found.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the account's email (compared without case).
type Request struct {
	Email string
}

// Response says what was done.
type Response struct {
	UserID uuid.UUID
	// Changed is false when the account already was an operator.
	Changed bool
	// HasPassword is true once the account's password is set.
	HasPassword bool
}
