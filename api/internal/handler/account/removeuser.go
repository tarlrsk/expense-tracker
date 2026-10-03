package account

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountremoveaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/removeaccount"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// RemoveUser handles DELETE /api/admin/users/{id} (operator): the user and all their data are
// deleted. The operator's own id and the last operator are 409; an id that is not a UUID is 404
// (ADR-0068).
func RemoveUser(p accountremoveaccountproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.MustCaller(c)
		if !ok {
			return
		}
		id, err := httpx.PathUUID(c, "id", domain.NoSuchUserMessage)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		if _, err := p.Execute(c.Request.Context(), accountremoveaccountproc.Request{UserID: id, OperatorID: caller.UserID}); err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
