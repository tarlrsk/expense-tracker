package transactions

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionsremoveproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/remove"
)

// Remove handles DELETE /api/transactions/{id} (authed): 204. An unknown id, another user's id,
// an id that is not a UUID and one already deleted are the same 404.
func Remove(p transactionsremoveproc.Processor) gin.HandlerFunc {
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
		if _, err := p.Execute(c.Request.Context(), transactionsremoveproc.Request{UserID: caller.UserID, ID: id}); err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
