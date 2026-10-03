// Package removeaccount deletes an account and, through the database's cascade, all of its data
// (ADR-0038, ADR-0068). It keeps the last-operator rule: the last operator can be removed neither
// by an operator nor by themselves.
package removeaccount

import (
	"context"

	"github.com/google/uuid"
)

// Processor removes accounts.
type Processor interface {
	// Execute removes the account, or returns not_found (no such user) or conflict (the last
	// operator, or an operator removing themselves through the admin path).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request names the account to remove.
type Request struct {
	UserID uuid.UUID
	// OperatorID is the operator removing someone through DELETE /api/admin/users/{id}; removing
	// their own account that way is refused (they use DELETE /api/me, which asks for the
	// password). uuid.Nil for DELETE /api/me.
	OperatorID uuid.UUID
}

// Response is empty: the account is gone.
type Response struct{}
