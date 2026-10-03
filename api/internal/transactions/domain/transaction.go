// Package domain holds the transactions module's entities and pure rules: the amount, date,
// text, currency, source and id rules, the list's period, page size and cursor, and the client
// messages (ADR-0039, ADR-0040, ADR-0041, ADR-0042, ADR-0071). It touches no database.
//
// No rule message quotes the value it refused: amounts, merchants, notes and dates are private
// and an error message can reach the request log.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Transaction is one of a user's transactions. OwnerID is kept and sent in every response (the
// groups guardrail of docs/05-roadmap.md); raw_input is not part of it until smart entry.
type Transaction struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Amount     Amount
	Currency   string
	OccurredOn Date
	Merchant   string
	CategoryID uuid.UUID
	Note       string
	Source     Source
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Client messages (ADR-0071).
const (
	// IDRuleMessage: a create without an id, with an id that is not a UUID version 7, or with
	// an id another user's transaction has. The last case must not be told apart from the
	// others (ADR-0040).
	IDRuleMessage = "id must be a new UUID version 7"
	// AmountRuleMessage: an amount that is not text, not a plain decimal, not more than 0, over
	// MaxAmount or with more than two decimals.
	AmountRuleMessage = `amount must be text such as "145.00": more than 0, at most 9999999999.99, with at most two decimals`
	// DateRuleMessage: an occurred_on that is not a real YYYY-MM-DD date in the allowed range.
	DateRuleMessage = "occurred_on must be a real date written YYYY-MM-DD, from 2000-01-01 to one year after today"
	// CategoryMessage: an unknown category, another user's, an archived one or a malformed id,
	// all alike (ADR-0071).
	CategoryMessage = "choose one of your active categories (create or unarchive one if there is none)"
	// CurrencyMessage: a currency other than THB (ADR-0041).
	CurrencyMessage = "currency must be THB"
	// SourceMessage: a source other than manual on create.
	SourceMessage = "source must be manual"
	// MerchantRuleMessage: a merchant that is too long or has a control character.
	MerchantRuleMessage = "merchant must be at most 100 characters, without control characters"
	// NoteRuleMessage: a note that is too long or has a control character other than a newline.
	NoteRuleMessage = "note must be at most 500 characters, without control characters other than line breaks"
	// NoChangeMessage: a PATCH body without any field.
	NoChangeMessage = "send at least one of amount, occurred_on, category_id, merchant, note or currency"
	// NotFoundMessage: an unknown id, another user's id, or an id that is not a UUID, alike.
	NotFoundMessage = "there is no such transaction"

	// PeriodMessage: a malformed month, from or to, month together with from or to, or from
	// after to.
	PeriodMessage = "use month (YYYY-MM), or from and to (YYYY-MM-DD, from not after to)"
	// FilterCategoryMessage: a category filter that is not a UUID.
	FilterCategoryMessage = "category must be a category id"
	// FilterSourceMessage: a source filter that is not one of the sources.
	FilterSourceMessage = "source must be one of manual, text, scan, csv"
	// LimitMessage: a limit that is not a whole number from 1 to MaxPageSize.
	LimitMessage = "limit must be a whole number from 1 to 200"
	// CursorMessage: a cursor the API did not make.
	CursorMessage = "the cursor is not valid; start again from the first page"
)
