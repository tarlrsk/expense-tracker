// Package parse reads the caller's quick entry text locally, without AI: the items, their
// amounts and days, and the category of each merchant the caller has a rule for
// (docs/03-modules.md §M4 and §M5, PLAN-0003 T3). It writes nothing.
package parse

import (
	"context"

	"github.com/google/uuid"

	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Processor parses quick entry text.
type Processor interface {
	// Execute reads the text into items (smartentry/domain.Parse) with today in the app time zone
	// (ADR-0042), and looks up the caller's merchant rule of every complete item whose merchant
	// has a key (ADR-0078). An item is resolved when its rule is found: it then carries the rule's
	// category and the merchant's name as stored on the rule (ADR-0077). Incomplete items are never
	// looked up. All lookups run in one user transaction, and none is opened when no item needs
	// one.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the text they typed.
type Request struct {
	UserID uuid.UUID
	Text   string
}

// Response is the items in the order written.
type Response struct {
	Items []Item
}

// Item is one item read from the text.
type Item struct {
	// Text is the item's part of the text, trimmed.
	Text string
	// Amount is the amount read; HasAmount is false when none could be read.
	Amount    transactionsdomain.Amount
	HasAmount bool
	// OccurredOn is the item's day; the zero Date when it could not be decided.
	OccurredOn transactionsdomain.Date
	// Merchant is the rule's stored name when the item is resolved, otherwise the merchant as
	// typed ("" when there is none or it breaks the merchant rules).
	Merchant string
	// CategoryID is the rule's category when the item is resolved, otherwise uuid.Nil.
	CategoryID uuid.UUID
	// Resolved is true when the item is complete and the caller has a rule for its merchant: it
	// needs no AI.
	Resolved bool
}
