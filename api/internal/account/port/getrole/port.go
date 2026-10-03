// Package getrole is the query that reads an account's role by user id.
package getrole

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Port reads roles.
type Port interface {
	// Get returns the role of the account userID; found is false when there is no users row. An
	// account without a profile counts as a plain user.
	Get(ctx context.Context, userID uuid.UUID) (role domain.Role, found bool, err error)
}
