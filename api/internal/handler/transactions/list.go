package transactions

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	transactionslistproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/list"
)

// listParams are the query parameters of GET /api/transactions; any other name is refused.
var listParams = []string{"month", "from", "to", "category", "source", "limit", "cursor"}

// List handles GET /api/transactions (authed): one page of the caller's transactions, newest
// first, and next_cursor while more exist (ADR-0071). An unknown, repeated or malformed
// parameter is 400.
func List(p transactionslistproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		q, err := httpx.Query(c, listParams...)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		param := func(name string) *string {
			if v, ok := q[name]; ok {
				return &v
			}
			return nil
		}
		resp, err := p.Execute(c.Request.Context(), transactionslistproc.Request{
			UserID: caller.UserID, Month: param("month"), From: param("from"), To: param("to"),
			Category: param("category"), Source: param("source"), Limit: param("limit"), Cursor: param("cursor"),
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newListResponse(resp.Transactions, resp.Next))
	}
}
