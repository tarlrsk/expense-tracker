package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountlistusersproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/listusers"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// listUsersResponse wraps the list, so paging fields can be added later without changing the
// shape (ADR-0068).
type listUsersResponse struct {
	Users []userItem `json:"users"`
}

// ListUsers handles GET /api/admin/users (operator).
func ListUsers(p accountlistusersproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := p.Execute(c.Request.Context(), accountlistusersproc.Request{})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		body := listUsersResponse{Users: make([]userItem, 0, len(resp.Users))}
		for _, u := range resp.Users {
			body.Users = append(body.Users, newUserItem(u))
		}
		c.JSON(http.StatusOK, body)
	}
}
