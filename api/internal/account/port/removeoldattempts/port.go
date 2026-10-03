// Package removeoldattempts is the query that deletes old login attempts (ADR-0066).
package removeoldattempts

import (
	"context"
	"time"
)

// Port deletes old login attempts.
type Port interface {
	// Remove deletes every login_attempts row made before before.
	Remove(ctx context.Context, before time.Time) error
}
