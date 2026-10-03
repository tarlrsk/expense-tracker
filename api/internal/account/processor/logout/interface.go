// Package logout ends the caller's current session (ADR-0025).
package logout

import (
	"context"

	"github.com/google/uuid"
)

// Processor logs out.
type Processor interface {
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request names the caller's current session (from the session check).
type Request struct {
	SessionID uuid.UUID
}

// Response is empty: the session is gone.
type Response struct{}
