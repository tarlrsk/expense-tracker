// Package find is the query that reads one of the caller's merchant rules by merchant key, for
// proposing a category (ADR-0078). Row-level security hides other users' rules, so another
// user's rule is simply not found.
//
// The smart entry module's local parser (smartentry/processor/parse, PLAN-0003 T3) uses it.
package find

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
)

// Port finds merchant rules.
type Port interface {
	// Find returns the rule of ownerID with merchantKey (domain.MerchantKey). found is false when
	// there is none, and also when the rule's category is archived: such a rule stays in the
	// table but is not used until the category is active again or a new choice replaces it.
	Find(ctx context.Context, ownerID uuid.UUID, merchantKey string) (r domain.Rule, found bool, err error)
}
