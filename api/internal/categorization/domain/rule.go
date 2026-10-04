// Package domain holds the categorization module's entities and pure rules: merchant rules and
// the merchant key (ADR-0077, ADR-0078). It touches no database.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Rule is one of a user's learned merchant rules: transactions whose merchant has MerchantKey are
// proposed in CategoryID. Merchant is the name as the user last wrote it. It has no owner field:
// rules stay per user (docs/05-roadmap.md groups guardrails), and every query runs for the caller
// only.
type Rule struct {
	ID          uuid.UUID
	MerchantKey string
	Merchant    string
	CategoryID  uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// MaxLength is the longest merchant name and merchant key of a rule, in characters (runes): the
// merchant limit of ADR-0071, checked by the merchant_rules table (migration 0005).
const MaxLength = 100
