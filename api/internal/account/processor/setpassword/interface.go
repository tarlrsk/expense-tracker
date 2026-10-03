// Package setpassword sets a password from an emailed set-password link (accept invite) and logs
// the person in (ADR-0025, ADR-0037, ADR-0066).
package setpassword

import (
	"context"
	"time"
)

// Processor sets a password from a link.
type Processor interface {
	// Execute returns a new session, or invalid_input: for a password that breaks the rule, and
	// with one message for every unusable link (unknown, used, expired, or a disabled user).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the link's token and the new password.
type Request struct {
	Token    string
	Password string
}

// Response is the new session.
type Response struct {
	Token     string
	ExpiresAt time.Time
}
