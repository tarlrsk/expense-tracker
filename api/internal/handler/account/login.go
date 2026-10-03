package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountloginproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/login"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /api/auth/login (public).
func Login(p accountloginproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := p.Execute(c.Request.Context(), accountloginproc.Request{
			Email: req.Email, Password: req.Password, IP: c.ClientIP(),
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newSessionResponse(resp.Token, resp.ExpiresAt))
	}
}
