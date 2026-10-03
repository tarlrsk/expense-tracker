// Package create adds a category at the end of the caller's list (POST /api/categories,
// ADR-0039, ADR-0061, ADR-0069).
package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// Processor creates categories.
type Processor interface {
	// Execute creates the category, or returns invalid_input (name, kind or icon rule) or
	// conflict (the name is taken by a non-archived category, or the caller already has
	// domain.MaxCategories categories).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the new category as sent; Execute applies the rules.
type Request struct {
	UserID uuid.UUID
	Name   string
	Kind   string
	Icon   string
}

// Response is the created category.
type Response struct {
	Category domain.Category
}
