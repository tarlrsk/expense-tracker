// Package updatedescriptionstyle is the query that sets the caller's own description style
// (ADR-0081). It runs as app_user inside WithUserTx, so row-level security and the column grant
// (description_style only) apply (ADR-0058, ADR-0068).
package updatedescriptionstyle

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Port sets description styles.
type Port interface {
	// Update writes profiles.description_style (that column only) of userID and reports whether a
	// row was written; row-level security hides every profile but the transaction user's.
	Update(ctx context.Context, userID uuid.UUID, style domain.DescriptionStyle) (bool, error)
}
