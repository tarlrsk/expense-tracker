// Package transactions holds the gin handlers of the transactions module, one file per endpoint.
package transactions

import (
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// transactionItem is one transaction in every response of /api/transactions (ADR-0071). owner_id
// is always there, so the UI can later show who spent what (docs/05-roadmap.md groups
// guardrails). amount is text with exactly two decimals, so no client turns it into a float;
// occurred_on is YYYY-MM-DD. raw_input is not sent until smart entry.
type transactionItem struct {
	ID         uuid.UUID `json:"id"`
	OwnerID    uuid.UUID `json:"owner_id"`
	Amount     string    `json:"amount"`
	Currency   string    `json:"currency"`
	OccurredOn string    `json:"occurred_on"`
	Merchant   string    `json:"merchant"`
	CategoryID uuid.UUID `json:"category_id"`
	Note       string    `json:"note"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func newTransactionItem(t domain.Transaction) transactionItem {
	return transactionItem{
		ID: t.ID, OwnerID: t.OwnerID, Amount: t.Amount.String(), Currency: t.Currency,
		OccurredOn: t.OccurredOn.String(), Merchant: t.Merchant, CategoryID: t.CategoryID, Note: t.Note,
		Source: string(t.Source), CreatedAt: t.CreatedAt.UTC(), UpdatedAt: t.UpdatedAt.UTC(),
	}
}

// listResponse is the body of GET /api/transactions. next_cursor is null on the last page.
type listResponse struct {
	Transactions []transactionItem `json:"transactions"`
	NextCursor   *string           `json:"next_cursor"`
}

func newListResponse(ts []domain.Transaction, next *domain.Cursor) listResponse {
	body := listResponse{Transactions: make([]transactionItem, 0, len(ts))}
	for _, t := range ts {
		body.Transactions = append(body.Transactions, newTransactionItem(t))
	}
	if next != nil {
		s := next.String()
		body.NextCursor = &s
	}
	return body
}
