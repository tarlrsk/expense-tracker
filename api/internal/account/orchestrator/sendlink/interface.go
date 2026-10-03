// Package sendlink is the operator's "send a new set-password link"
// (POST /api/admin/users/{id}/set-password-link, ADR-0025, ADR-0038, ADR-0068). It is also the
// operator-driven password reset: it works for a user with or without a password, cancels their
// earlier links and does not end their sessions.
package sendlink

import (
	"context"

	"github.com/google/uuid"
)

// Orchestrator sends links.
type Orchestrator interface {
	// Execute returns whether the email went out, or not_found. A failed email is not an error:
	// the new link exists and the operator can try again.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request names the user.
type Request struct {
	UserID uuid.UUID
}

// Response says whether the email was sent.
type Response struct {
	EmailSent bool
}
