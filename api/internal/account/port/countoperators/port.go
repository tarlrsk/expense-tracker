// Package countoperators is the query that counts operator profiles, for the last-operator rule
// (ADR-0038, ADR-0056).
package countoperators

import "context"

// Port counts operators.
type Port interface {
	// Count returns the number of profiles with the operator role, including accounts whose
	// invite is not yet accepted (ADR-0068).
	Count(ctx context.Context) (int, error)
}
