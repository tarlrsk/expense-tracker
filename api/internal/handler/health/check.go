// Package health holds the gin handlers of the health module.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	healthcheckproc "github.com/tarlrsk/expense-tracker/api/internal/health/processor/check"
)

type checkResponse struct {
	Status string `json:"status"`
}

// Check handles GET /api/healthz.
func Check(p healthcheckproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := p.Execute(c.Request.Context(), healthcheckproc.Request{}); err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.JSON(http.StatusOK, checkResponse{Status: "ok"})
	}
}
