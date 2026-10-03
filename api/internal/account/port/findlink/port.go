// Package findlink is the query that checks whether a set-password link can be used.
package findlink

import (
	"context"
	"time"
)

// Port checks links.
type Port interface {
	// Usable reports whether a set_password email token with that hash exists, is unused, has
	// not expired at now, and belongs to a user who is not disabled. It takes no lock.
	Usable(ctx context.Context, tokenHash []byte, now time.Time) (bool, error)
}
