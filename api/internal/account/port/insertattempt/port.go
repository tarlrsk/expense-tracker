// Package insertattempt is the query that records one failed (or not yet successful) login.
package insertattempt

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// Port records login attempts.
type Port interface {
	// Insert adds a login_attempts row and returns its id.
	Insert(ctx context.Context, email string, ip netip.Addr, at time.Time) (uuid.UUID, error)
}
