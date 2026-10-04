// Package upsert is the query that writes one of the caller's merchant rules: it adds the rule
// for a merchant key or replaces the merchant name and category of the one that exists (the
// latest choice wins, ADR-0076, ADR-0078). It runs as app_user inside WithUserTx; the row-level
// security policy refuses a row for anyone but the transaction user, and the update names only
// the columns app_user may change (ADR-0058).
package upsert

import (
	"context"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
)

// NewRule is the rule to write; every field is already checked: MerchantKey is
// domain.MerchantKey of Merchant and not empty, Merchant is trimmed and not empty, both have at
// most domain.MaxLength characters, and CategoryID is one of the owner's categories.
type NewRule struct {
	OwnerID     uuid.UUID
	MerchantKey string
	Merchant    string
	CategoryID  uuid.UUID
}

// Port writes merchant rules.
type Port interface {
	// Upsert adds the rule or, when the owner already has one with the key, sets its merchant
	// and category; it returns the rule as stored. Two upserts of one key at the same moment
	// leave one row: the second waits for the first and then replaces it.
	Upsert(ctx context.Context, r NewRule) (domain.Rule, error)
}
