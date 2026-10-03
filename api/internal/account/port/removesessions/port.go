// Package removesessions is the query that deletes a user's sessions, optionally keeping one.
package removesessions

import (
	"context"

	"github.com/google/uuid"
)

// Port deletes sessions.
type Port interface {
	// Remove deletes every session of userID except keep; uuid.Nil keeps none.
	Remove(ctx context.Context, userID, keep uuid.UUID) error
}
