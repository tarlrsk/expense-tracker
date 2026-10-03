// Package listusers is the query behind the operator's user list (ADR-0068). It reads account
// tables only, so it can never return financial data (ADR-0019, ADR-0034).
package listusers

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Port lists users.
type Port interface {
	// List returns every account ordered by created_at, then id. LastActiveAt is the latest
	// sessions.last_used_at of the user, or nil.
	List(ctx context.Context) ([]domain.Account, error)
}
