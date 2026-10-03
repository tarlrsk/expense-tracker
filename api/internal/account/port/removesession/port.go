// Package removesession is the query that deletes one session (logout).
package removesession

import (
	"context"

	"github.com/google/uuid"
)

// Port deletes a session.
type Port interface {
	// Remove deletes the session with id sessionID; a missing session is not an error.
	Remove(ctx context.Context, sessionID uuid.UUID) error
}
