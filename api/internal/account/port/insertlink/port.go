// Package insertlink is the query that stores a new set-password link (ADR-0037).
package insertlink

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NewLink is a link to store. Only the token's hash is stored (ADR-0066).
type NewLink struct {
	UserID    uuid.UUID
	TokenHash []byte
	// Now is created_at.
	Now       time.Time
	ExpiresAt time.Time
}

// Port stores links.
type Port interface {
	// Insert adds an unused set_password email token.
	Insert(ctx context.Context, l NewLink) error
}
