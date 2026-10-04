package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// ResolvedBy says what settled a proposal of POST /api/entry/parse (ADR-0076, PLAN-0003 T5).
type ResolvedBy string

// The ways a proposal is settled.
const (
	// ResolvedByRule: the user's merchant rule gave the category.
	ResolvedByRule ResolvedBy = "rule"
	// ResolvedByAI: the AI read the item.
	ResolvedByAI ResolvedBy = "ai"
	// ResolvedByNone: only the local parser read the item, and it is not complete.
	ResolvedByNone ResolvedBy = "none"
)

// AIStatus says whether, and how, the AI took part in a parse.
type AIStatus string

// The AI statuses.
const (
	// AINotNeeded: every item was resolved locally; the AI was not called.
	AINotNeeded AIStatus = "not_needed"
	// AIUsed: the AI read the unresolved items.
	AIUsed AIStatus = "used"
	// AILimitReached: the user's daily AI limit is used up; the AI was not called.
	AILimitReached AIStatus = "limit_reached"
	// AIUnavailable: the AI was called and failed (down, too slow, an unusable answer).
	AIUnavailable AIStatus = "unavailable"
	// AINotConfigured: no AI key is set; the AI was not called.
	AINotConfigured AIStatus = "not_configured"
)

// Limits of a quick entry text (PLAN-0003 T5).
const (
	// MaxTextLength is the longest text, in characters (runes), once trimmed.
	MaxTextLength = 1000
	// MaxItems is how many items one text may hold, as the local parser splits it.
	MaxItems = 20
)

// Client messages of POST /api/entry/parse. None quotes the text.
const (
	// TextRuleMessage: a blank text or one over MaxTextLength.
	TextRuleMessage = "text must be 1 to 1,000 characters"
	// NoItemMessage: a text with nothing to read, such as a date word only.
	NoItemMessage = `text must hold at least one item, such as "coffee 60"`
	// TooManyItemsMessage: a text of more than MaxItems items.
	TooManyItemsMessage = "text must hold at most 20 items"
)

// CheckText trims text and checks it holds 1 to MaxTextLength characters; otherwise it returns
// an invalid_input error.
func CheckText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || utf8.RuneCountInString(text) > MaxTextLength {
		return "", apperr.New(apperr.InvalidInput, TextRuleMessage)
	}
	return text, nil
}
