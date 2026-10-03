// Package setrole is the query that sets a profile's role. Only the operator command uses it
// (ADR-0035, ADR-0056); no API endpoint sets a role.
package setrole

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Port sets roles.
type Port interface {
	// Set writes profiles.role (that column only) for userID and reports whether it changed:
	// false when the profile already had role, or does not exist.
	Set(ctx context.Context, userID uuid.UUID, role domain.Role) (changed bool, err error)
}
