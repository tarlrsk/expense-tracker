package categories

import (
	"net/http"

	"github.com/gin-gonic/gin"

	categorieslistproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/list"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// List handles GET /api/categories (authed): the caller's categories, archived ones included, in
// the caller's order.
func List(p categorieslistproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		resp, err := p.Execute(c.Request.Context(), categorieslistproc.Request{UserID: caller.UserID})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newListResponse(resp.Categories))
	}
}
