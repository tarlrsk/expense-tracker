// Package checkpassword checks a logged-in user's password before a sensitive action (delete my
// account), counted and limited exactly like a failed login (ADR-0037, ADR-0066, ADR-0068).
package checkpassword

import (
	"context"

	"github.com/google/uuid"
)

// Processor checks a password.
type Processor interface {
	// Execute returns nil when the password is the user's, or one of: invalid_input (wrong
	// password; the failure is counted), rate_limited (too many failures for the email or the
	// address; the password is not checked), unauthenticated (the account is gone).
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
