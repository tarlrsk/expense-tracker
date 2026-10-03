// Package getprofile reads the caller's own profile (GET /api/me).
package getprofile

import (
	"context"

	"github.com/google/uuid"

	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
)

// Processor reads a profile.
type Processor interface {
	// Execute returns the caller's profile, or unauthenticated when the account is gone.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller, from the session check.
type Request struct {
	UserID uuid.UUID
}

// Response is the profile.
type Response struct {
	Profile accountgetprofileport.Profile
}
