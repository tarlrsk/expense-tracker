// Package insert is the query that creates a category at the end of the user's list (ADR-0061,
// ADR-0069). It runs as app_user inside WithUserTx; the row-level security policy refuses a row
// for anyone but the transaction user.
package insert

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// ErrNameTaken: another non-archived category of the user has the name, ignoring case. Two
// inserts racing for one name give one success and this error.
var ErrNameTaken = errors.New("insert category: the name is taken")

// NewCategory is the category to create; Name and Icon are already normalised.
type NewCategory struct {
	OwnerID uuid.UUID
	Name    string
	Icon    string
	Kind    domain.Kind
}

// Port creates categories.
type Port interface {
	// Insert adds the category with sort_order one more than the largest of the owner's
	// categories (archived ones included), computed in the same statement, and returns it. It
	// returns ErrNameTaken when the unique name index refuses the row.
	Insert(ctx context.Context, c NewCategory) (domain.Category, error)
}
