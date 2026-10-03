package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountgetprofileproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/getprofile"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// GetMe handles GET /api/me (authed): the caller's own profile.
func GetMe(p accountgetprofileproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		resp, err := p.Execute(c.Request.Context(), accountgetprofileproc.Request{UserID: caller.UserID})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newProfileResponse(resp.Profile))
	}
}
