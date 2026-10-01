package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// Recover turns a panic into a 500 internal response and logs it with the stack.
func Recover(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			if err, ok := r.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(r)
			}
			logger.LogAttrs(c.Request.Context(), slog.LevelError, "panic",
				slog.String("request_id", c.GetString(RequestIDKey)),
				slog.String("panic", fmt.Sprint(r)),
				slog.String("stack", string(debug.Stack())),
			)
			httpx.WriteError(c, apperr.Wrap(apperr.Internal, "", fmt.Errorf("panic: %v", r)))
		}()
		c.Next()
	}
}
