// Package updateprofile changes the caller's own profile (PATCH /api/me): the display name only.
// A user can never change their role or email (ADR-0056, ADR-0068).
package updateprofile

import (
	"context"

	"github.com/google/uuid"

	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
)

// Processor updates a profile.
type Processor interface {
	// Execute writes the fields that are set and returns the whole profile, or invalid_input
	// (display name too long) or unauthenticated (the account is gone).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the fields to change; a nil field is left as it is.
type Request struct {
	UserID      uuid.UUID
	DisplayName *string
}

// Response is the profile after the change.
type Response struct {
	Profile accountgetprofileport.Profile
}
