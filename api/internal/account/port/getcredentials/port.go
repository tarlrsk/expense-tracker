// Package getcredentials is the query that reads a logged-in user's email and password hash.
package getcredentials

import (
	"context"

	"github.com/google/uuid"
)

// Credentials are what a password change checks.
type Credentials struct {
	Email        string
	PasswordHash string
}

// Port reads credentials.
type Port interface {
	// Get returns the credentials of userID; found is false when the account does not exist.
	Get(ctx context.Context, userID uuid.UUID) (c Credentials, found bool, err error)
}
