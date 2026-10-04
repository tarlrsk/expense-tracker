// Package reserve is the query that counts one AI parse toward a user's daily limit, before the
// AI is called (ai_usage, ADR-0077, PLAN-0003 T5). It runs as app_user inside WithUserTx; the
// row-level security policy refuses a row for anyone but the transaction user.
package reserve

import (
	"context"

	"github.com/google/uuid"

	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Port reserves AI parses.
type Port interface {
	// Reserve adds one to ownerID's parse count of day when the count is below limit, and returns
	// the new count with reserved true. At the limit (and always when limit is 0 or less) it
	// writes nothing and returns reserved false. It is one statement, so two requests at once
	// cannot both take the last use. day is a day in the app time zone (ADR-0042).
	Reserve(ctx context.Context, ownerID uuid.UUID, day transactionsdomain.Date, limit int) (count int, reserved bool, err error)
}
