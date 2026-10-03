// Package list returns the caller's categories (GET /api/categories): all of them, archived ones
// included and flagged, in the caller's order (ADR-0061, ADR-0069).
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// Processor lists categories.
type Processor interface {
	// Execute returns the caller's categories ordered by sort_order, then id.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller.
type Request struct {
	UserID uuid.UUID
}

// Response is the list.
type Response struct {
	Categories []domain.Category
}
