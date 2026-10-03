// Package find is the query that reads one of the caller's categories by id. Row-level security
// hides other users' rows, so another user's id is simply not found (ADR-0069).
package find

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// Port finds categories.
type Port interface {
	// Find returns the category id of ownerID; found is false when there is none.
	Find(ctx context.Context, ownerID, id uuid.UUID) (c domain.Category, found bool, err error)
}
