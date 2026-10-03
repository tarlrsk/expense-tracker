// Package removeattempts is the query that deletes every login attempt for an email (after a
// successful login, ADR-0066).
package removeattempts

import "context"

// Port deletes an email's login attempts.
type Port interface {
	// Remove deletes the login_attempts rows of email (compared without case), from any address.
	Remove(ctx context.Context, email string) error
}
