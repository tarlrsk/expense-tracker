// Package update renames a category, sets its icon, archives or unarchives it (PATCH
// /api/categories/{id}, ADR-0039, ADR-0069). The kind never changes.
package update

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// Processor updates categories.
type Processor interface {
	// Execute applies the fields that are set and returns the category after the change, or
	// invalid_input (no field, or a name or icon rule), not_found (no such category of the
	// caller) or conflict (the name would be taken by a non-archived category).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller, the category and the fields to change; a nil field is left as it is.
type Request struct {
	UserID   uuid.UUID
	ID       uuid.UUID
	Name     *string
	Icon     *string
	Archived *bool
}

// Response is the category after the change.
type Response struct {
	Category domain.Category
}
