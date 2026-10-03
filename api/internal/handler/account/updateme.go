package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountupdateprofileproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/updateprofile"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// updateMeRequest has the only field a user may change; any other field (role, email ...) is
// refused by DecodeJSON. A missing or null display_name leaves it as it is.
type updateMeRequest struct {
	DisplayName *string `json:"display_name"`
}

// UpdateMe handles PATCH /api/me (authed): it returns the profile after the change.
func UpdateMe(p accountupdateprofileproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		var req updateMeRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := p.Execute(c.Request.Context(), accountupdateprofileproc.Request{
			UserID: caller.UserID, DisplayName: req.DisplayName,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newProfileResponse(resp.Profile))
	}
}
