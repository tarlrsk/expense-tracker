// Package findcredentials is the query that finds an account's login details by email.
package findcredentials

import (
	"context"

	"github.com/google/uuid"
)

// Credentials are what a login checks.
type Credentials struct {
	UserID uuid.UUID
	// PasswordHash is "" until the invite is accepted.
	PasswordHash string
	Disabled     bool
}

// Port finds credentials.
type Port interface {
	// Find returns the account with that email (compared without case); found is false when
	// there is none.
	Find(ctx context.Context, email string) (c Credentials, found bool, err error)
}
