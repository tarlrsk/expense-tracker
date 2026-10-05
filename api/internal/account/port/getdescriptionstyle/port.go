// Package getdescriptionstyle is the query that reads the caller's own description style
// (ADR-0081). It runs as app_user inside WithUserTx, so row-level security limits it to the
// caller's profile. It is a read port of the account module for later modules: smart entry reads
// it in the transaction that reserves an AI parse (ADR-0032, PLAN-0003 T7).
package getdescriptionstyle

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Port reads description styles.
type Port interface {
	// Get returns profiles.description_style of userID; found is false when there is no profile
	// the transaction user can see.
	Get(ctx context.Context, userID uuid.UUID) (style domain.DescriptionStyle, found bool, err error)
}
