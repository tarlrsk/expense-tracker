// Package findsession is the query that finds a live session by its token hash.
package findsession

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Session is a live session with its user's role.
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Role       domain.Role
	LastUsedAt time.Time
}

// Port finds a session.
type Port interface {
	// Find returns the session whose token hash is tokenHash, only if it has not expired at now
	// and its user is not disabled. found is false otherwise.
	Find(ctx context.Context, tokenHash []byte, now time.Time) (s Session, found bool, err error)
}
