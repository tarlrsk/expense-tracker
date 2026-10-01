// Package apperr holds the error kinds shared by all layers. It knows nothing about HTTP.
package apperr

import (
	"context"
	"errors"
)

// Kind classifies an error. Its string value is the error code sent to clients.
type Kind string

// The error kinds.
const (
	InvalidInput    Kind = "invalid_input"
	Unauthenticated Kind = "unauthenticated"
	Forbidden       Kind = "forbidden"
	NotFound        Kind = "not_found"
	Conflict        Kind = "conflict"
	RateLimited     Kind = "rate_limited"
	Timeout         Kind = "timeout"
	Internal        Kind = "internal"
)

// Valid reports whether k is one of the kinds above.
func (k Kind) Valid() bool {
	switch k {
	case InvalidInput, Unauthenticated, Forbidden, NotFound, Conflict, RateLimited, Timeout, Internal:
		return true
	default:
		return false
	}
}

// Error is an error with a kind and a message that is safe to show to the client.
// The wrapped cause is for logs only.
type Error struct {
	Kind    Kind
	Message string
	Err     error
}

func (e *Error) Error() string {
	s := string(e.Kind)
	if e.Message != "" {
		s += ": " + e.Message
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

func (e *Error) Unwrap() error { return e.Err }

// New returns an error of the given kind with a client-safe message.
func New(kind Kind, msg string) error {
	return &Error{Kind: kind, Message: msg}
}

// Wrap returns an error of the given kind with a client-safe message that wraps err.
func Wrap(kind Kind, msg string, err error) error {
	return &Error{Kind: kind, Message: msg, Err: err}
}

// KindOf returns the kind of err. An *Error in the chain decides; otherwise
// context.DeadlineExceeded is Timeout and anything else is Internal. KindOf(nil) is "".
func KindOf(err error) Kind {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		if e.Kind.Valid() {
			return e.Kind
		}
		return Internal
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	return Internal
}
