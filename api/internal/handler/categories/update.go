package categories

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	categoriesupdateproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/update"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// updateRequest has the fields a PATCH may change; a missing or null field is left as it is.
// kind is not here, so DecodeJSON refuses it: a category's kind never changes (ADR-0039).
type updateRequest struct {
	Name     *string `json:"name"`
	Icon     *string `json:"icon"`
	Archived *bool   `json:"archived"`
}

// Update handles PATCH /api/categories/{id} (authed): 200 with the category after the change. An
// unknown id, another user's id and an id that is not a UUID are the same 404 (ADR-0069).
func Update(p categoriesupdateproc.Processor) gin.HandlerFunc {
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
		resp, err := p.Execute(c.Request.Context(), categoriesupdateproc.Request{
			UserID: caller.UserID, ID: id, Name: req.Name, Icon: req.Icon, Archived: req.Archived,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newCategoryItem(resp.Category))
	}
}
