// Package setorder is the query that writes a user's new category order (PUT
// /api/categories/order, ADR-0061, ADR-0069).
package setorder

import (
	"context"

	"github.com/google/uuid"
)

// Port writes the order.
type Port interface {
	// SetOrder gives the categories ids of ownerID the sort_order 1 to len(ids), in the order
	// given. Categories not listed keep theirs; a listed category already at its number is not
	// written. ids must not repeat an id.
	SetOrder(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) error
}
