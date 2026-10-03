// Package removeattempt is the query that deletes one login attempt by id (the attempt turned
// out to be a success).
package removeattempt

import (
	"context"

	"github.com/google/uuid"
)

// Port deletes a login attempt.
type Port interface {
	// Remove deletes the login_attempts row with that id.
	Remove(ctx context.Context, id uuid.UUID) error
}
