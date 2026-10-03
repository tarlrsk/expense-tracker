package categories

import (
	"net/http"

	"github.com/gin-gonic/gin"

	categoriescreateproc "github.com/tarlrsk/expense-tracker/api/internal/categories/processor/create"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// createRequest is the body of POST /api/categories. Any other field (owner_id, sort_order ...) is
// refused by DecodeJSON. A missing icon is none.
type createRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Icon string `json:"icon"`
}

// Create handles POST /api/categories (authed): 201 with the new category, which is last in the
// list.
func Create(p categoriescreateproc.Processor) gin.HandlerFunc {
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
		resp, err := p.Execute(c.Request.Context(), categoriescreateproc.Request{
			UserID: caller.UserID, Name: req.Name, Kind: req.Kind, Icon: req.Icon,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusCreated, newCategoryItem(resp.Category))
	}
}
