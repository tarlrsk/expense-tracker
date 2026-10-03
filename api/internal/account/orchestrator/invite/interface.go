// Package invite is the operator's invite (POST /api/admin/invites, ADR-0025, ADR-0068): create
// the account and its set-password link, commit, then email the link.
package invite

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Orchestrator invites.
type Orchestrator interface {
	// Execute returns the new account and whether the email went out, or invalid_input (not one
	// plain address) or conflict (the email has an account). A failed email is not an error: the
	// account exists and the operator can send a new link.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the email to invite.
type Request struct {
	Email string
}

// Response is the new account. EmailSent is false when the email could not be sent.
type Response struct {
	Account   domain.Account
	EmailSent bool
}
