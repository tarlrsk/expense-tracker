// Package list is the query that reads a user's categories (GET /api/categories, ADR-0069). It
// runs as app_user inside WithUserTx, so row-level security limits it to the caller's rows.
package list

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// Port lists categories.
type Port interface {
	// List returns every category of ownerID, archived ones included, ordered by sort_order,
	// then id (ADR-0061).
	List(ctx context.Context, ownerID uuid.UUID) ([]domain.Category, error)
}
