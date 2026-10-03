// Package changepassword changes the logged-in user's password (ADR-0037, ADR-0066).
package changepassword

import (
	"context"

	"github.com/google/uuid"
)

// Processor changes a password.
type Processor interface {
	// Execute changes the password and ends the user's other sessions, or returns
	// invalid_input (new password breaks the rule, or the current password is wrong) or
	// rate_limited (too many failures, counted like failed logins).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request comes from an authed caller.
type Request struct {
	UserID    uuid.UUID
	SessionID uuid.UUID // kept; every other session of the user ends
	// IP is the client's address, for the failure limit.
	IP              string
	CurrentPassword string
	NewPassword     string
}

// Response is empty.
type Response struct{}
