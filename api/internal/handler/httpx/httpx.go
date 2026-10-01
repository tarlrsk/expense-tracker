// Package httpx maps errors to HTTP responses and holds shared response helpers.
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// Fixed client messages for kinds whose cause must never reach the client.
const (
	InternalMessage = "something went wrong"
	TimeoutMessage  = "the request took too long"
)

var statusByKind = map[apperr.Kind]int{
	apperr.InvalidInput:    http.StatusBadRequest,
	apperr.Unauthenticated: http.StatusUnauthorized,
	apperr.Forbidden:       http.StatusForbidden,
	apperr.NotFound:        http.StatusNotFound,
	apperr.Conflict:        http.StatusConflict,
	apperr.RateLimited:     http.StatusTooManyRequests,
	apperr.Timeout:         http.StatusGatewayTimeout,
	apperr.Internal:        http.StatusInternalServerError,
}

// ErrorBody is the JSON body of every error response.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the code and client-safe message of an error response.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StatusOf returns the HTTP status for an error kind; unknown kinds are 500.
func StatusOf(kind apperr.Kind) int {
	if status, ok := statusByKind[kind]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// WriteError records err on the gin context for the request log, writes the error
// body (unless a response was already written) and aborts the handler chain.
func WriteError(c *gin.Context, err error) {
	if err == nil {
		err = errors.New("httpx: WriteError called with a nil error")
	}
	_ = c.Error(err)

	if c.Writer.Written() {
		c.Abort()
		return
	}

	kind := apperr.KindOf(err)
	c.AbortWithStatusJSON(StatusOf(kind), ErrorBody{Error: ErrorDetail{Code: string(kind), Message: clientMessage(kind, err)}})
}

func clientMessage(kind apperr.Kind, err error) string {
	switch kind {
	case apperr.Internal:
		return InternalMessage
	case apperr.Timeout:
		return TimeoutMessage
	}
	var e *apperr.Error
	if errors.As(err, &e) && e.Message != "" {
		return e.Message
	}
	return http.StatusText(StatusOf(kind))
}
