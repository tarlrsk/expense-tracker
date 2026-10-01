// Package middleware holds the gin middleware shared by every route.
package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Deadline puts a timeout on the request context (ADR-0043). Handlers pass the
// context on and return its error; httpx.WriteError turns that into 504 timeout.
func Deadline(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
