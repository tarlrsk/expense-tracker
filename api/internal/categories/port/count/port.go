// Package count is the query that counts a user's categories, for the limit (ADR-0069).
package count

import (
	"context"

	"github.com/google/uuid"
)

// Port counts categories.
type Port interface {
	// Count returns how many categories ownerID has, archived ones included.
	Count(ctx context.Context, ownerID uuid.UUID) (int, error)
}
