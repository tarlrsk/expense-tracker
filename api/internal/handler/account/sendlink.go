package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountsendlinkorch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/sendlink"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// SendLink handles POST /api/admin/users/{id}/set-password-link (operator): 200 with whether the
// email was sent. An id that is not a UUID is 404, like an unknown one (ADR-0068).
func SendLink(o accountsendlinkorch.Orchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := httpx.PathUUID(c, "id", domain.NoSuchUserMessage)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		resp, err := o.Execute(c.Request.Context(), accountsendlinkorch.Request{UserID: id})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, emailSentResponse{EmailSent: resp.EmailSent})
	}
}
