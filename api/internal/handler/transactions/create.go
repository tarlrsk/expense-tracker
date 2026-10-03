package transactions

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	transactionscreateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/create"
)

// createRequest is the body of POST /api/transactions. Any other field (owner_id, raw_input ...)
// is refused by DecodeJSON, and so is an amount sent as a JSON number: it must be text
// (ADR-0071). A missing merchant or note is empty; a missing (or null) currency is THB and a
// missing source is manual.
type createRequest struct {
	ID         string  `json:"id"`
	Amount     string  `json:"amount"`
	OccurredOn string  `json:"occurred_on"`
	CategoryID string  `json:"category_id"`
	Merchant   string  `json:"merchant"`
	Note       string  `json:"note"`
	Currency   *string `json:"currency"`
	Source     *string `json:"source"`
}

// Create handles POST /api/transactions (authed): 201 with the new transaction, or 200 with the
// stored one when the caller already has a transaction with the id (ADR-0040).
func Create(p transactionscreateproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		var req createRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := p.Execute(c.Request.Context(), transactionscreateproc.Request{
			UserID: caller.UserID, ID: req.ID, Amount: req.Amount, OccurredOn: req.OccurredOn,
			CategoryID: req.CategoryID, Merchant: req.Merchant, Note: req.Note, Currency: req.Currency, Source: req.Source,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		status := http.StatusOK
		if resp.Created {
			status = http.StatusCreated
		}
		c.JSON(status, newTransactionItem(resp.Transaction))
	}
}
