// Package checksession is the session check: it turns a bearer token into the caller's identity
// (ADR-0025, ADR-0037, ADR-0066). The session middleware calls it on every authed request.
package checksession

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Processor checks a session token.
type Processor interface {
	// Execute returns the caller, or an unauthenticated error when the token is missing,
	// malformed, unknown or expired, or its user is disabled (all with the same message).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the bearer token as the client sent it.
type Request struct {
	Token string
}

// Response is the caller's identity. The role is read on every request, so a role change takes
// effect at once.
type Response struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      domain.Role
}
