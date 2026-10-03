package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountsetpasswordproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/setpassword"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

type setPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// SetPassword handles POST /api/auth/set-password (public): the token comes from the emailed
// link; the person is logged in at once (ADR-0066).
func SetPassword(p accountsetpasswordproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req setPasswordRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := p.Execute(c.Request.Context(), accountsetpasswordproc.Request{Token: req.Token, Password: req.Password})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newSessionResponse(resp.Token, resp.ExpiresAt))
	}
}
