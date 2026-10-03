// Package login is the email and password login (ADR-0025, ADR-0037, ADR-0066).
package login

import (
	"context"
	"time"
)

// Processor logs in.
type Processor interface {
	// Execute returns a new session, or one of: invalid_input (no usable email), rate_limited
	// (too many failures for the email or the address), unauthenticated (unknown email, wrong
	// password, invite not accepted or disabled account, all alike).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is a login attempt.
type Request struct {
	Email    string
	Password string
	// IP is the client's address (the direct peer; ADR-0066).
	IP string
}

// Response is the new session. Token is sent to the client once and never stored.
type Response struct {
	Token     string
	ExpiresAt time.Time
}
