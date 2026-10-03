// Package updatepassword is the query that writes a user's password hash.
package updatepassword

import (
	"context"

	"github.com/google/uuid"
)

// Update is a password hash to write.
type Update struct {
	UserID uuid.UUID
	Hash   string
	// IfHash, when set, makes the write happen only while the stored hash still equals it (a
	// password change checked against the hash it verified).
	IfHash *string
}

// Port writes password hashes.
type Port interface {
	// Update writes users.password_hash (that column only, ADR-0058) and reports whether a row
	// was written.
	Update(ctx context.Context, u Update) (bool, error)
}
