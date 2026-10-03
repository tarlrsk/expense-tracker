// Package remove is the query that deletes one of the caller's transactions. Row-level security
// hides other users' rows, so another user's id deletes nothing.
package remove

import (
	"context"

	"github.com/google/uuid"
)

// Port deletes transactions.
type Port interface {
	// Remove deletes transaction id of ownerID; removed is false when there was none.
	Remove(ctx context.Context, ownerID, id uuid.UUID) (removed bool, err error)
}
