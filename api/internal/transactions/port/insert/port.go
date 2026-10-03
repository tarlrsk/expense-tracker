// Package insert is the query that creates a transaction with the id the client made (ADR-0040).
// It runs as app_user inside WithUserTx; the row-level security policy refuses a row for anyone
// but the transaction user.
package insert

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// ErrCategory: the owner-scoped foreign key refused the category (it is not one of the owner's).
// The processor checks the category first; this is the last line of defence.
var ErrCategory = errors.New("insert transaction: the category is not the owner's")

// NewTransaction is the transaction to create; every field is already checked.
type NewTransaction struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Amount     domain.Amount
	Currency   string
	OccurredOn domain.Date
	Merchant   string
	CategoryID uuid.UUID
	Note       string
	Source     domain.Source
}

// Port creates transactions.
type Port interface {
	// Insert adds the transaction and returns it as stored, with inserted true. When a row with
	// the id already exists, of the caller or of another user (hidden by row-level security), it
	// writes nothing and returns inserted false; an insert racing another with the same id waits
	// for it and then does the same. It returns ErrCategory when the foreign key refuses the
	// category.
	Insert(ctx context.Context, t NewTransaction) (stored domain.Transaction, inserted bool, err error)
}
