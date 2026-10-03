// Package reorder puts the caller's non-archived categories in a new order (PUT
// /api/categories/order, ADR-0061, ADR-0069).
package reorder

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// Processor reorders categories.
type Processor interface {
	// Execute writes the new order and returns the whole list in it, or invalid_input (no ids,
	// or an id listed twice) or conflict (the ids are not exactly the caller's non-archived
	// categories; nothing is written).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and every non-archived category id in the new order. IDs is nil when the
// client sent none; an empty list is valid when every category is archived.
type Request struct {
	UserID uuid.UUID
	IDs    []uuid.UUID
}

// Response is the list after the change, archived categories included, like GET returns it.
type Response struct {
	Categories []domain.Category
}
