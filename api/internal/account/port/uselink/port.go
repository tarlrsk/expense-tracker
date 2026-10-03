// Package uselink is the query that uses up a set-password link, once.
package uselink

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Port uses links.
type Port interface {
	// Use marks the set_password email token with that hash as used at now and returns its
	// user, only if it was unused, has not expired at now and its user is not disabled; ok is
	// false otherwise. It is one UPDATE, so the row lock makes a second, concurrent call wait and
	// then find the link used.
	Use(ctx context.Context, tokenHash []byte, now time.Time) (userID uuid.UUID, ok bool, err error)
}
