// Package touchsession is the query that records a session's use and extends it.
package touchsession

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Touch says how to record one use of a session.
type Touch struct {
	SessionID uuid.UUID
	// Now becomes last_used_at, ExpiresAt becomes expires_at.
	Now       time.Time
	ExpiresAt time.Time
	// StaleBefore: the row is written only when its last_used_at is earlier than this.
	StaleBefore time.Time
}

// Port records session use.
type Port interface {
	// Touch writes last_used_at and expires_at in one conditional update; it does nothing when
	// the session was used at or after t.StaleBefore.
	Touch(ctx context.Context, t Touch) error
}
