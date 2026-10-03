// Package lock is the query that locks all of a user's categories until the transaction ends.
// Every write of the categories module takes it first, so two writes of one user run one after
// the other: the limit, the end of the list and the reorder check see each other's result
// (ADR-0069).
package lock

import (
	"context"

	"github.com/google/uuid"
)

// Port locks categories.
type Port interface {
	// Lock takes a row lock on every category of ownerID, archived ones included, and holds it
	// until the transaction commits or rolls back.
	Lock(ctx context.Context, ownerID uuid.UUID) error
}
