package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// MaxBodyBytes is the largest JSON request body DecodeJSON reads.
const MaxBodyBytes = 64 << 10

// Client messages of DecodeJSON. The decoder's own text is never used: it can quote part of the
// body, which may be a password.
const (
	malformedBodyMessage = "the request body is not valid JSON for this endpoint"
	bodyTooLargeMessage  = "the request body is too large"
)

// DecodeJSON reads the request body as exactly one JSON value into dst. A body larger than
// MaxBodyBytes, malformed JSON, a field dst does not have, a value of the wrong type or anything
// after the value is an invalid_input error.
func DecodeJSON(c *gin.Context, dst any) error {
	if c.Request.Body == nil {
		return apperr.New(apperr.InvalidInput, malformedBodyMessage)
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return bodyError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return bodyError(err)
		}
		return apperr.New(apperr.InvalidInput, malformedBodyMessage)
	}
	return nil
}

func bodyError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return apperr.New(apperr.InvalidInput, bodyTooLargeMessage)
	}
	return apperr.New(apperr.InvalidInput, malformedBodyMessage)
}

// Caller is the logged-in caller, set by the session middleware on authed and operator routes.
// Handlers pass UserID into processor requests; they never take a user id from the client.
type Caller struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	// Role is the profile's role, read on this request ("user" or "operator").
	Role string
}

const callerKey = "httpx.caller"

// SetCaller records the caller on the gin context. Only the session middleware calls it.
func SetCaller(c *gin.Context, caller Caller) { c.Set(callerKey, caller) }

// CallerOf returns the caller recorded by the session middleware; ok is false on a route
// without it.
func CallerOf(c *gin.Context) (Caller, bool) {
	v, ok := c.Get(callerKey)
	if !ok {
		return Caller{}, false
	}
	caller, ok := v.(Caller)
	return caller, ok && caller.UserID != uuid.Nil
}

// MustCaller is CallerOf for handlers on authed routes: without a caller (a route registered on
// the wrong group) it writes an internal error and returns false.
func MustCaller(c *gin.Context) (Caller, bool) {
	caller, ok := CallerOf(c)
	if !ok {
		WriteError(c, errors.New("httpx: no caller on an authed route; register it on routes.Authed"))
	}
	return caller, ok
}

// PathUUID returns the path parameter name as a UUID. A value that is not a UUID is a not_found
// error with notFoundMessage, the same answer as an unknown id (ADR-0068).
func PathUUID(c *gin.Context, name, notFoundMessage string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, apperr.New(apperr.NotFound, notFoundMessage)
	}
	return id, nil
}
