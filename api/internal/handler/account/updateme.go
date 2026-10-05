package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountupdateprofileproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/updateprofile"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// updateMeRequest has the only fields a user may change; any other field (role, email ...) is
// refused by DecodeJSON. A missing or null field leaves it as it is.
type updateMeRequest struct {
	DisplayName *string `json:"display_name"`
	// DescriptionStyle is as_typed, light_tidy or clean_name (ADR-0081).
	DescriptionStyle *string `json:"description_style"`
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
			UserID: caller.UserID, DisplayName: req.DisplayName, DescriptionStyle: req.DescriptionStyle,
		})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, newProfileResponse(resp.Profile))
	}
}
