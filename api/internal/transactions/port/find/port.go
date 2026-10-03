// Package find is the query that reads one of the caller's transactions by id. Row-level
// security hides other users' rows, so another user's id is simply not found.
package find

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Port finds transactions.
type Port interface {
	// Find returns the transaction id of ownerID; found is false when there is none.
	Find(ctx context.Context, ownerID, id uuid.UUID) (t domain.Transaction, found bool, err error)
}
