// Package update is the query that changes one of the caller's categories: its name, icon or
// archived flag, and its place when it is unarchived (ADR-0069). It names only the columns it
// changes; the database grants app_user nothing else (ADR-0058).
package update

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
)

// ErrNameTaken: the change would give the category the name of another non-archived category of
// the user, ignoring case (a rename or an unarchive).
var ErrNameTaken = errors.New("update category: the name is taken")

// ErrNoChange: Update was called with nothing to change.
var ErrNoChange = errors.New("update category: nothing to change")

// Changes are the columns to write; a nil field is left as it is. Name and Icon are already
// normalised.
type Changes struct {
	Name     *string
	Icon     *string
	Archived *bool
	// ToEnd sets sort_order to one more than the largest of the owner's categories.
	ToEnd bool
}

// Port changes categories.
type Port interface {
	// Update writes the changes to category id of ownerID and returns the category after them;
	// found is false when there is no such category. It returns ErrNameTaken when the unique
	// name index refuses the change.
	Update(ctx context.Context, ownerID, id uuid.UUID, ch Changes) (c domain.Category, found bool, err error)
}
