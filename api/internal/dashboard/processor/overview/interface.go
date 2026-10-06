// Package overview returns the caller's dashboard for one month: the month's totals, its
// spending by category, the six-month trend and the top descriptions, never transaction rows
// (GET /api/dashboard, ADR-0087, ADR-0088).
package overview

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
)

// Processor builds the dashboard.
type Processor interface {
	// Execute returns the four blocks of the requested month, or of the current month in the app
	// time zone when none was sent; invalid_input for a malformed or out-of-range month.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the month parameter as sent; a nil Month was not sent.
type Request struct {
	UserID uuid.UUID
	Month  *string
}

// Response is the month the blocks cover and the blocks. Categories and TopDescriptions are
// empty for a month without spending; Trend always has domain.TrendMonths rows, oldest first,
// ending with Month.
type Response struct {
	Month           domain.Month
	Totals          domain.MonthTotals
	Categories      []domain.CategoryTotal
	Trend           []domain.MonthSpending
	TopDescriptions []domain.DescriptionTotal
}
