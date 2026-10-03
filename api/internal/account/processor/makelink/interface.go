// Package makelink is the one path that makes a set-password link (ADR-0037, ADR-0068): it
// cancels the user's earlier links and stores a new one. It sends nothing; the orchestrator that
// calls it emails the link after the transaction has committed (ADR-0032).
package makelink

import (
	"context"

	"github.com/google/uuid"
)

// Processor makes links.
type Processor interface {
	// Execute makes a new link for an existing user, with or without a password, or returns
	// not_found. It does not end the user's sessions: setting the new password does.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request names the user.
type Request struct {
	UserID uuid.UUID
}

// Response is where to send the link and its token. The token is secret: it goes only into the
// email, never into a log or an API response.
type Response struct {
	Email string
	Token string
}
