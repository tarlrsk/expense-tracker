// Package check is the health module's liveness use case.
package check

import "context"

// Processor runs the liveness check.
type Processor interface {
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the input of the liveness check (none yet).
type Request struct{}

// Response is the result of a passing liveness check.
type Response struct{}
