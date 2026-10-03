package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountchangepasswordproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/changepassword"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword handles POST /api/me/password (authed). A wrong current password is 400, not
// 401: the web client reads 401 as "logged out" (ADR-0066).
func ChangePassword(p accountchangepasswordproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		var req changePasswordRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		_, err := p.Execute(c.Request.Context(), accountchangepasswordproc.Request{
			UserID: caller.UserID, SessionID: caller.SessionID, IP: c.ClientIP(),
			CurrentPassword: req.CurrentPassword, NewPassword: req.NewPassword,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
