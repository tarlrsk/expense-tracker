package domain

import (
	"strings"

	"github.com/google/uuid"

	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Confidence is how sure a proposal is. Low ones are highlighted for the user (ADR-0076).
type Confidence string

// The confidence levels.
const (
	ConfidenceHigh Confidence = "high"
	ConfidenceLow  Confidence = "low"
)

// AIItem is one item as the AI read it, before any check. Its fields are text, as the AI wrote
// them.
type AIItem struct {
	// Text is the part of the user's text the item came from.
	Text string
	// Amount is the amount in baht, such as "145.50"; "" for none.
	Amount string
	// OccurredOn is the day as YYYY-MM-DD; "" for none.
	OccurredOn string
	// Merchant is the description; "" for none.
	Merchant string
	// CategoryRef is the number of the chosen category in the list sent to the AI; 0 for none.
	CategoryRef int
	// Confidence is the AI's own confidence.
	Confidence Confidence
}

// Proposal is one checked item, for the user to confirm. A field that failed its check is empty.
type Proposal struct {
	// Text is the part of the user's text the item came from, trimmed.
	Text string
	// Amount is the amount; HasAmount is false when the AI gave none or it broke the amount rules.
	Amount    transactionsdomain.Amount
	HasAmount bool
	// OccurredOn is the day; the zero Date when the AI gave none or it was out of range.
	OccurredOn transactionsdomain.Date
	// Merchant is the description, trimmed; "" when the AI gave none or it broke the merchant
	// rules.
	Merchant string
	// CategoryID is the chosen category; uuid.Nil when none was chosen or the number was not in
	// the list.
	CategoryID uuid.UUID
	// Confidence is the AI's confidence when every field passed, otherwise ConfidenceLow.
	Confidence Confidence
}

// CheckAIItem checks one AI item against the transaction rules (ADR-0071) and returns its
// proposal. today is the current day in the app time zone (ADR-0042); categories maps each number
// sent to the AI to the user's active category it stands for.
//
// Nothing the user typed is dropped (PLAN-0003 T4): a field that fails its check is left empty
// and the proposal becomes low. A missing field (no amount, day, merchant or category) also makes
// it low, since the user must fill it in. Only a proposal with every field valid keeps the AI's
// confidence; an unknown confidence is low.
func CheckAIItem(item AIItem, today transactionsdomain.Date, categories map[int]uuid.UUID) Proposal {
	p := Proposal{Text: strings.TrimSpace(item.Text)}
	if a, err := transactionsdomain.ParseAmount(strings.TrimSpace(item.Amount)); err == nil {
		p.Amount, p.HasAmount = a, true
	}
	if d, err := transactionsdomain.ParseOccurredOn(strings.TrimSpace(item.OccurredOn), today); err == nil {
		p.OccurredOn = d
	}
	if m, err := transactionsdomain.NormalizeMerchant(item.Merchant); err == nil && m != "" {
		p.Merchant = m
	}
	if id, ok := categories[item.CategoryRef]; ok && item.CategoryRef != 0 && id != uuid.Nil {
		p.CategoryID = id
	}
	p.Confidence = confidence(item.Confidence, p)
	return p
}

// ApplyRule gives a checked proposal the category of the user's merchant rule (docs/03-modules.md
// §M5: the rule comes first) and works its confidence out again as CheckAIItem does, from item's
// own confidence: a proposal that was low only for want of a category may now keep the AI's
// confidence. A nil category changes nothing.
func ApplyRule(item AIItem, p Proposal, category uuid.UUID) Proposal {
	if category == uuid.Nil {
		return p
	}
	p.CategoryID = category
	p.Confidence = confidence(item.Confidence, p)
	return p
}

// confidence is ai when every field of p is filled and ai is ConfidenceHigh, otherwise
// ConfidenceLow.
func confidence(ai Confidence, p Proposal) Confidence {
	complete := p.HasAmount && !p.OccurredOn.IsZero() && p.Merchant != "" && p.CategoryID != uuid.Nil
	if complete && ai == ConfidenceHigh {
		return ConfidenceHigh
	}
	return ConfidenceLow
}
