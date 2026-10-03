// Package deleteme deletes the caller's own account and all its data (DELETE /api/me,
// ADR-0038, ADR-0068): the password is checked first, counted and limited like a failed login,
// then the account is removed under the last-operator rule.
package deleteme

import (
	"context"

	"github.com/google/uuid"
)

// Orchestrator deletes the caller's account.
type Orchestrator interface {
	// Execute returns nil when the account is gone, or one of: invalid_input (wrong password),
	// rate_limited, conflict (the last operator), unauthenticated.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request comes from an authed caller.
type Request struct {
	UserID uuid.UUID
	// IP is the client's address, for the failure limit.
	IP       string
	Password string
}

// Response is empty.
type Response struct{}
