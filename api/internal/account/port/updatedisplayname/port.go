// Package updatedisplayname is the query that renames the caller's own profile. It runs as
// app_user inside WithUserTx, so row-level security and the column grant (display_name only)
// apply (ADR-0058, ADR-0068).
package updatedisplayname

import (
	"context"

	"github.com/google/uuid"
)

// Port renames profiles.
type Port interface {
	// Update writes profiles.display_name (that column only) of userID and reports whether a row
	// was written; row-level security hides every profile but the transaction user's.
	Update(ctx context.Context, userID uuid.UUID, displayName string) (bool, error)
}
