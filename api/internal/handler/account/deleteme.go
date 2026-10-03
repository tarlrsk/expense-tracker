package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountdeletemeorch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/deleteme"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

type deleteMeRequest struct {
	Password string `json:"password"`
}

// DeleteMe handles DELETE /api/me (authed): the caller's account and all its data are deleted.
// A wrong password is 400, not 401, like change password (ADR-0066, ADR-0068).
func DeleteMe(o accountdeletemeorch.Orchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		var req deleteMeRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		_, err := o.Execute(c.Request.Context(), accountdeletemeorch.Request{
			UserID: caller.UserID, IP: c.ClientIP(), Password: req.Password,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
