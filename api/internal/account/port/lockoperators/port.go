// Package lockoperators is the lock that serialises account removals, so the last-operator check
// and the removal cannot interleave with another removal (ADR-0038, ADR-0068).
package lockoperators

import "context"

// Port takes the lock.
type Port interface {
	// Lock waits for and takes the lock. It is a transaction-level lock: it is released when the
	// transaction ends, never kept on a pooled connection.
	Lock(ctx context.Context) error
}
