// Package lock is the query that reads one of the caller's transactions by id and locks its row
// until the user transaction ends, so an update or a delete decides on what it will change and
// two of them on one row run one after the other. Row-level security hides other users' rows,
// so another user's id is simply not found.
package lock

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Port locks transactions.
type Port interface {
	// Lock locks and returns the transaction id of ownerID; found is false when there is none.
	Lock(ctx context.Context, ownerID, id uuid.UUID) (t domain.Transaction, found bool, err error)
}
