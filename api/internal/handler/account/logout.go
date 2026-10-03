package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountlogoutproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/logout"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// Logout handles POST /api/auth/logout (authed): it ends the session whose token was sent.
func Logout(p accountlogoutproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		if _, err := p.Execute(c.Request.Context(), accountlogoutproc.Request{SessionID: caller.SessionID}); err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
