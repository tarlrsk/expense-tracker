// Package insertuser is the query that creates a users row (ADR-0036). Only the createaccount
// processor uses it: one place creates accounts (docs/05-roadmap.md guardrails).
package insertuser

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrEmailTaken: another account already has this email (compared without case). Two inserts
// racing for one email give one success and this error.
var ErrEmailTaken = errors.New("insert user: the email already has an account")

// NewUser is the created row.
type NewUser struct {
	ID        uuid.UUID
	CreatedAt time.Time
}

// Port creates users.
type Port interface {
	// Insert adds a users row without a password (an invite not yet accepted). The database
	// trigger seeds the user's default categories in the same transaction (ADR-0036). It returns
	// ErrEmailTaken when the email is in use.
	Insert(ctx context.Context, email string) (NewUser, error)
}
