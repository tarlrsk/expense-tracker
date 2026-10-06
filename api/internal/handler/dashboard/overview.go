package dashboard

import (
	"net/http"

	"github.com/gin-gonic/gin"

	dashboardoverviewproc "github.com/tarlrsk/expense-tracker/api/internal/dashboard/processor/overview"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// overviewParams are the query parameters of GET /api/dashboard; any other name is refused.
var overviewParams = []string{"month"}

// Overview handles GET /api/dashboard (authed): the caller's totals, category breakdown, trend
// and top descriptions for one month, the current one in the app time zone by default
// (ADR-0088, ADR-0089). An unknown, repeated or malformed parameter is 400.
func Overview(p dashboardoverviewproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		q, err := httpx.Query(c, overviewParams...)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		req := dashboardoverviewproc.Request{UserID: caller.UserID}
		if v, ok := q["month"]; ok {
			req.Month = &v
		}
		resp, err := p.Execute(c.Request.Context(), req)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newOverviewResponse(resp))
	}
}
