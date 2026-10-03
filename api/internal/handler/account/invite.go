package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	accountinviteorch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/invite"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

type inviteRequest struct {
	Email string `json:"email"`
}

type inviteResponse struct {
	User      userItem `json:"user"`
	EmailSent bool     `json:"email_sent"`
}

// Invite handles POST /api/admin/invites (operator): 201 with the new account, also when the
// email could not be sent (email_sent false; ADR-0068).
func Invite(o accountinviteorch.Orchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req inviteRequest
		if err := httpx.DecodeJSON(c, &req); err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := o.Execute(c.Request.Context(), accountinviteorch.Request{Email: req.Email})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusCreated, inviteResponse{User: newUserItem(resp.Account), EmailSent: resp.EmailSent})
	}
}
