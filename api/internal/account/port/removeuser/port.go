// Package removeuser is the query that deletes an account. Deleting the users row as app_auth
// cascades to every row of the user, financial tables included (ADR-0034; cascade_test.go).
package removeuser

import (
	"context"

	"github.com/google/uuid"
)

// Port deletes accounts.
type Port interface {
	// Remove deletes the users row of userID and reports whether there was one.
	Remove(ctx context.Context, userID uuid.UUID) (bool, error)
}
