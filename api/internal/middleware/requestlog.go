package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestIDKey is the gin context key holding the request id.
const RequestIDKey = "request_id"

// RequestLog writes one log line per request after it finishes. It sets a fresh
// X-Request-Id (any incoming one is ignored) and never logs headers, bodies or
// the query string.
func RequestLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		id := newRequestID()
		c.Set(RequestIDKey, id)
		c.Header("X-Request-Id", id)

		c.Next()

		status := c.Writer.Status()
		attrs := []slog.Attr{
			slog.String("request_id", id),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
		}
		if last := c.Errors.Last(); last != nil {
			attrs = append(attrs, slog.String("error", last.Err.Error()))
		}
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		logger.LogAttrs(c.Request.Context(), level, "request", attrs...)
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	// crypto/rand.Read never returns an error; it crashes the program instead.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
