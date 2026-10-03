package categories

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	categoriesreorderproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/reorder"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// reorderRequest is the body of PUT /api/categories/order: every non-archived category id once,
// in the new order. An id that is not a UUID makes the body malformed (invalid_input).
type reorderRequest struct {
	IDs []uuid.UUID `json:"ids"`
}

// Reorder handles PUT /api/categories/order (authed): 200 with the whole list in the new order,
// or 409 when the list does not match the caller's non-archived categories (ADR-0069).
func Reorder(p categoriesreorderproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		var req reorderRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := p.Execute(c.Request.Context(), categoriesreorderproc.Request{UserID: caller.UserID, IDs: req.IDs})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newListResponse(resp.Categories))
	}
}
