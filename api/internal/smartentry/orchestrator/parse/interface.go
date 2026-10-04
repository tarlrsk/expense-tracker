// Package parse is quick entry (POST /api/entry/parse, PLAN-0003 T5): the local parser first,
// then the AI for what it could not resolve, then the caller's merchant rules over the AI's
// reading (docs/03-modules.md §M4 and §M5, ADR-0076). Nothing is saved but the AI usage count.
package parse

import (
	"context"

	"github.com/google/uuid"

	aiparse "github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
	"github.com/tarlrsk/expense-tracker/api/internal/smartentry/domain"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Orchestrator reads quick entry text into proposals for the user to confirm.
type Orchestrator interface {
	// Execute returns one proposal per item, in the order typed. It returns invalid_input for a
	// blank text, one over 1,000 characters, or one the local parser reads as no item or as
	// more than 20. Without the AI (not needed, not configured, the daily limit reached, or a
	// failed call) the items still come back with what the local parser read, and AI says why;
	// only the request's own deadline makes a failed call an error (timeout).
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller, the text they typed and the description style for the AI ("" is
// as typed; PLAN-0003 T7 passes the caller's setting).
type Request struct {
	UserID uuid.UUID
	Text   string
	Style  aiparse.Style
}

// Response is the proposals in the order typed, and what the AI did.
type Response struct {
	Items []Item
	AI    domain.AIStatus
}

// Item is one proposal. A field that could not be read is empty.
type Item struct {
	// Text is the item's part of the text.
	Text string
	// Amount is the amount; HasAmount is false when none could be read.
	Amount    transactionsdomain.Amount
	HasAmount bool
	// OccurredOn is the day; the zero Date when it could not be read.
	OccurredOn transactionsdomain.Date
	// Merchant is the description; "" for none.
	Merchant string
	// CategoryID is one of the caller's active categories; uuid.Nil for none.
	CategoryID uuid.UUID
	// Confidence is high or low; low ones are highlighted for the user.
	Confidence domain.Confidence
	// ResolvedBy says what gave the category: a rule, the AI, or nothing.
	ResolvedBy domain.ResolvedBy
}
