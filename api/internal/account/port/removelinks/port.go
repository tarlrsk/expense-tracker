// Package removelinks is the query that deletes a user's set-password links: a new link cancels
// the earlier ones (ADR-0037, ADR-0068).
package removelinks

import (
	"context"

	"github.com/google/uuid"
)

// Port deletes links.
type Port interface {
	// Remove deletes every set_password email token of userID, used, expired or not.
	Remove(ctx context.Context, userID uuid.UUID) error
}
