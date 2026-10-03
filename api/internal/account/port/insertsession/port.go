// Package insertsession is the query that creates a login session.
package insertsession

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NewSession is a session to create. Only the token's hash is stored.
type NewSession struct {
	UserID    uuid.UUID
	TokenHash []byte
	// Now is created_at and last_used_at.
	Now       time.Time
	ExpiresAt time.Time
}

// Port creates sessions.
type Port interface {
	// Insert creates the session and returns its id.
	Insert(ctx context.Context, s NewSession) (uuid.UUID, error)
}
