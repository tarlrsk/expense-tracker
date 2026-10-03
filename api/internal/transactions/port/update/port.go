// Package update is the query that changes one of the caller's transactions: its amount,
// currency, date, merchant, category or note. It names only the columns it changes; the database
// grants app_user nothing else (ADR-0058).
package update

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// ErrCategory: the owner-scoped foreign key refused the new category (it is not one of the
// owner's). The processor checks the category first; this is the last line of defence.
var ErrCategory = errors.New("update transaction: the category is not the owner's")

// ErrNoChange: Update was called with nothing to change.
var ErrNoChange = errors.New("update transaction: nothing to change")

// Changes are the columns to write; a nil field is left as it is. Every value is already
// checked.
type Changes struct {
	Amount     *domain.Amount
	Currency   *string
	OccurredOn *domain.Date
	Merchant   *string
	CategoryID *uuid.UUID
	Note       *string
}

// Port changes transactions.
type Port interface {
	// Update writes the changes to transaction id of ownerID and returns it after them; found
	// is false when there is no such transaction. It returns ErrCategory when the foreign key
	// refuses the category.
	Update(ctx context.Context, ownerID, id uuid.UUID, ch Changes) (t domain.Transaction, found bool, err error)
}
