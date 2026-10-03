package transactions

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionsupdateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/update"
)

// updateRequest has the fields a PATCH may change; a missing or null field is left as it is.
// id, owner_id, source and raw_input are not here, so DecodeJSON refuses them: they never change
// (ADR-0058, ADR-0071).
type updateRequest struct {
	Amount     *string `json:"amount"`
	OccurredOn *string `json:"occurred_on"`
	CategoryID *string `json:"category_id"`
	Merchant   *string `json:"merchant"`
	Note       *string `json:"note"`
	Currency   *string `json:"currency"`
}

// Update handles PATCH /api/transactions/{id} (authed): 200 with the transaction after the
// change. An unknown id, another user's id and an id that is not a UUID are the same 404.
func Update(p transactionsupdateproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		id, err := httpx.PathUUID(c, "id", domain.NotFoundMessage)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		var req updateRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := p.Execute(c.Request.Context(), transactionsupdateproc.Request{
			UserID: caller.UserID, ID: id, Amount: req.Amount, OccurredOn: req.OccurredOn, CategoryID: req.CategoryID,
			Merchant: req.Merchant, Note: req.Note, Currency: req.Currency,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newTransactionItem(resp.Transaction))
	}
}
