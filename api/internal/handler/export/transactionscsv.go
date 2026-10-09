// Package export has the HTTP handlers of the export module (M8).
package export

import (
	"net/http"

	"github.com/gin-gonic/gin"

	exporttransactionscsvproc "github.com/tarlrsk/expense-tracker/api/internal/export/processor/transactionscsv"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// transactionsCSVParams are the query parameters of GET /api/export.csv; any other name is
// refused.
var transactionsCSVParams = []string{"from", "to"}

// TransactionsCSV handles GET /api/export.csv (authed): the caller's transactions as a CSV file
// to download, for all time or the period from and to (ADR-0087, ADR-0092). An unknown, repeated
// or malformed parameter is 400, as JSON like every error; only a success is CSV.
func TransactionsCSV(p exporttransactionscsvproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		q, err := httpx.Query(c, transactionsCSVParams...)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		req := exporttransactionscsvproc.Request{UserID: caller.UserID}
		if v, ok := q["from"]; ok {
			req.From = &v
		}
		if v, ok := q["to"]; ok {
			req.To = &v
		}
		resp, err := p.Execute(c.Request.Context(), req)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		// The file name holds only ASCII letters, digits and dashes, so it needs no escaping.
		c.Header("Content-Disposition", `attachment; filename="`+resp.FileName+`"`)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", resp.Body)
	}
}
