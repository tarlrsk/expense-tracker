// Package reserve starts an AI parse of quick entry text: it counts one use toward the caller's
// daily limit and reads the categories the AI may choose from (PLAN-0003 T5, ADR-0077). The AI is
// called afterwards, outside any transaction (ADR-0032), so a failed call still counts.
package reserve

import (
	"context"

	"github.com/google/uuid"

	categoriesdomain "github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Processor reserves AI parses.
type Processor interface {
	// Execute counts one AI parse for the caller on today in the app time zone (ADR-0042), when
	// the count is below AI_DAILY_PARSE_LIMIT, and then reads the caller's active categories, in
	// one user transaction. At the limit it writes and reads nothing and returns Reserved false.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller.
type Request struct {
	UserID uuid.UUID
}

// Response says whether a use was reserved and, when it was, the caller's active categories in
// the caller's order. Day is the day counted, today in the app time zone, either way.
type Response struct {
	Reserved   bool
	Day        transactionsdomain.Date
	Categories []categoriesdomain.Category
}
