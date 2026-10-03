// Package getprofile is the query that reads a user's own profile with their email. It runs as
// app_auth because the email is in users, which app_user cannot read (ADR-0068).
package getprofile

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Profile is what GET /api/me returns.
type Profile struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Role        domain.Role
	CreatedAt   time.Time
}

// Port reads profiles.
type Port interface {
	// Get returns the profile of userID; found is false when there is none.
	Get(ctx context.Context, userID uuid.UUID) (p Profile, found bool, err error)
}
