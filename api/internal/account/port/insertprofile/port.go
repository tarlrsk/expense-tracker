// Package insertprofile is the query that creates a user's profile row (ADR-0036). Only the
// createaccount processor uses it.
package insertprofile

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Port creates profiles.
type Port interface {
	// Insert adds the profile of userID with role and an empty display name.
	Insert(ctx context.Context, userID uuid.UUID, role domain.Role) error
}
